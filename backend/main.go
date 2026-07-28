// main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	cfg, err := LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if cfg.Alerts.CheckIntervalSeconds <= 0 {
		cfg.Alerts.CheckIntervalSeconds = 60
	}
	if cfg.Alerts.MinLateSeconds < 0 {
		cfg.Alerts.MinLateSeconds = 0
	}
	if cfg.Alerts.CooldownSeconds < 0 {
		cfg.Alerts.CooldownSeconds = 0
	}

	if cfg.PublicBaseURL == "" {
		cfg.PublicBaseURL = "https://runguard.smega.eu"
	}

	db, err := InitMongo(cfg)
	if err != nil {
		log.Fatalf("init mongo: %v", err)
	}

	app := &App{
		cfg:                  cfg,
		db:                   db,
		usersColl:            db.Collection("users"),
		tokensColl:           db.Collection("tokens"),
		jobsColl:             db.Collection("jobs"),
		jobRunsColl:          db.Collection("job_runs"),
		projectsColl:         db.Collection("projects"),
		incidentsColl:        db.Collection("incidents"),
		webhookLogsColl:      db.Collection("webhook_logs"), // NEW
		webhookTemplatesColl: db.Collection("webhook_templates"),
		adminAuditColl:       db.Collection("admin_audit_log"),
	}

	// Promote any configured bootstrap admins. Idempotent — safe to run
	// on every startup.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		bootstrapAdmins(ctx, app)
		cancel()
	}

	// Create index for job_runs (job_id + at desc)
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := app.jobRunsColl.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "job_id", Value: 1},
				{Key: "at", Value: -1},
			},
		})
		if err != nil {
			log.Printf("create job_runs index: %v", err)
		}
	}
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := app.webhookTemplatesColl.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "name", Value: 1},
			},
		})
		if err != nil {
			log.Printf("create webhook_templates index: %v", err)
		}
	}

	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := app.projectsColl.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "slug", Value: 1}},
		})
		if err != nil {
			log.Printf("create projects index: %v", err)
		}
	}
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := app.incidentsColl.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "state", Value: 1}, {Key: "updated_at", Value: -1}},
		})
		if err != nil {
			log.Printf("create incidents index: %v", err)
		}
	}

	// NEW: Create index for webhook_logs (job_id + created_at desc)
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := app.webhookLogsColl.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "job_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		})
		if err != nil {
			log.Printf("create webhook_logs index: %v", err)
		}
	}

	// start alert loop
	go app.RunAlertLoop()

	// start daily summary loop
	go app.RunDailySummaryLoop()

	// start trial-expiry loop
	go app.RunTrialExpiryLoop()

	r := gin.Default()

	// Only trust X-Forwarded-For from configured reverse proxies (e.g. a
	// local nginx). Without this, gin trusts every upstream by default,
	// which means any client could spoof that header and get a different
	// bucket in the per-IP rate limiter — i.e. bypass it entirely. An
	// empty list (the default) means "trust nobody", which is safe but
	// means everything behind a proxy is seen as one IP; set this to your
	// proxy's address once you have one.
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Fatalf("invalid server.trusted_proxies: %v", err)
	}

	// CORS
	r.Use(func(c *gin.Context) {
		origin := cfg.Server.FrontendOrigin
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(200)
			return
		}
		c.Next()
	})

	// Infra-facing health check (load balancer / systemd / external uptime
	// monitor). Deliberately outside /api and unauthenticated.
	r.GET("/healthz", app.handleHealthz)

	// Public heartbeat endpoint (no auth).
	// Rate limited two ways: a generous per-IP cap (so one host running many
	// jobs isn't penalized) and a tighter per-key cap (so a single job key
	// can't be flooded/guessed regardless of source IP).
	heartbeatByIP := rateLimitByIP(120, 120)
	heartbeatByKey := rateLimitByParam("key", 20, 5)
	r.GET("/hb/:key", heartbeatByIP, heartbeatByKey, app.handleHeartbeat)
	r.POST("/hb/:key", heartbeatByIP, heartbeatByKey, app.handleHeartbeat)

	api := r.Group("/api")

	// Auth routes. Rate limited per-IP since these are the endpoints most
	// exposed to brute-force/credential-stuffing and email-bombing abuse.
	auth := api.Group("/auth")
	{
		auth.POST("/signup", rateLimitByIP(5, 5), app.handleSignup)
		auth.POST("/login", rateLimitByIP(10, 10), app.handleLogin)
		auth.POST("/logout", app.handleLogout)
		auth.GET("/me", app.authMiddleware(), app.handleMe)

		auth.POST("/magic/request", rateLimitByIP(5, 5), app.handleMagicRequest)
		auth.POST("/magic/consume", rateLimitByIP(10, 10), app.handleMagicConsume)

		auth.POST("/password/reset/request", rateLimitByIP(5, 5), app.handlePasswordResetRequest)
		auth.POST("/password/reset/confirm", rateLimitByIP(10, 10), app.handlePasswordResetConfirm)
	}

	account := api.Group("/account").Use(app.authMiddleware())
	{
		account.GET("", app.handleGetAccount)
		account.PATCH("", app.handleUpdateAccount)
		account.POST("/logout-all", app.handleLogoutAllDevices)

		// Agent token: lets the CLI self-register jobs without the account
		// owner needing to create a heartbeat key by hand first. Viewed and
		// regenerated from the browser (cookie auth); used by the CLI as a
		// Bearer token against /api/agent/*.
		account.GET("/agent-token", rateLimitByIP(20, 10), app.handleGetAgentToken)
		account.POST("/agent-token/regenerate", rateLimitByIP(5, 5), app.handleRegenerateAgentToken)
	}

	// CLI self-registration: authenticated via "Authorization: Bearer
	// <agent_token>" instead of the browser session cookie — the CLI has no
	// way to hold a cookie jar across cron runs. Rate limiting runs BEFORE
	// the auth check (not after, like most other groups here) since this
	// route's only gate is a bearer token arriving in a header rather than
	// an already-established session cookie — the thing worth throttling
	// is exactly the brute-force-a-random-token attempt, which needs the
	// limiter to fire before the auth middleware would otherwise abort the
	// chain for an invalid token.
	agent := api.Group("/agent").Use(rateLimitByIP(20, 10), app.agentAuthMiddleware())
	{
		agent.POST("/register", app.handleAgentRegister)
	}

	// Admin-only user management/audit log. Replaces manually flipping
	// is_admin in mongosh: promote/demote here is logged to
	// admin_audit_log, and the last remaining admin can't be demoted.
	admin := api.Group("/admin").Use(app.authMiddleware(), app.requireAdmin())
	{
		admin.GET("/users", app.handleAdminListUsers)
		admin.POST("/users/:id/promote", app.handleAdminPromoteUser)
		admin.POST("/users/:id/demote", app.handleAdminDemoteUser)
		admin.GET("/audit-log", app.handleAdminAuditLog)
	}

	api.GET("/pricing", app.handlePublicPricing)
	api.POST("/billing/webhooks/stripe", app.handleBillingStripeWebhook)
	billing := api.Group("/billing").Use(app.authMiddleware())
	{
		billing.GET("", app.handleGetBilling)
		billing.POST("/checkout", app.handleCreateCheckoutLink)
		billing.POST("/portal", app.handleCreatePortalLink)
	}

	projects := api.Group("/projects").Use(app.authMiddleware())
	{
		projects.GET("", app.handleListProjects)
		projects.POST("", app.handleCreateProject)
		projects.PATCH("/:id", app.handleUpdateProject)
		projects.DELETE("/:id", app.handleDeleteProject)
	}

	incidents := api.Group("/incidents").Use(app.authMiddleware())
	{
		incidents.GET("", app.handleListIncidents)
		incidents.PATCH("/:id", app.handleUpdateIncident)
	}

	// Overview (authenticated)
	api.GET("/overview", app.authMiddleware(), app.handleOverview)

	// Jobs API (authenticated)
	jobs := api.Group("/jobs").Use(app.authMiddleware())
	{
		jobs.GET("", app.handleListJobs)
		jobs.POST("", app.handleCreateJob)
		jobs.PATCH("/:id", app.handleUpdateJob)
		jobs.DELETE("/:id", app.handleDeleteJob)

		// new: detail + run history
		jobs.GET("/:id", app.handleGetJob)
		jobs.GET("/:id/runs", app.handleListJobRuns)

		jobs.GET("/:id/stats", app.handleJobStats)

		jobs.POST("/:id/public", app.handleToggleJobPublic)

		// NEW: webhook logs for a job
		jobs.GET("/:id/webhook-logs", app.handleJobWebhookLogs)
		jobs.POST("/:id/test-webhooks", app.handleTestJobWebhooks)

		jobs.GET("/:id/runs/:runID/webhooks", app.ListWebhookLogsForRun)
	}

	// Exports (authenticated, top-tier only via plan.AllowExports)
	exports := api.Group("/exports").Use(app.authMiddleware())
	{
		exports.GET("/jobs.csv", app.handleExportJobsCSV)
		exports.GET("/jobs.xlsx", app.handleExportJobsExcel)
		exports.GET("/jobs.pdf", app.handleExportJobsPDF)
	}
	webhooks := api.Group("/webhook-templates").Use(app.authMiddleware())
	{
		webhooks.GET("", app.handleListWebhookTemplates)
		webhooks.POST("", app.handleCreateWebhookTemplate)
		webhooks.DELETE("/:id", app.handleDeleteWebhookTemplate)
	}

	// Public read-only API (no auth)
	public := api.Group("/public")
	{
		public.GET("/job/:token", app.handlePublicJob)
		public.GET("/cli/versions", app.handleCLIListVersions)
		public.GET("/status", app.handlePublicStatus)
	}

	srv := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: r,
	}

	// Serve in the background so we can watch for a shutdown signal below.
	go func() {
		log.Printf("listening on %s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown: on SIGINT/SIGTERM (Ctrl-C, or `systemctl stop`),
	// stop accepting new connections and give in-flight requests — an
	// export streaming out, a webhook send, an alert loop tick — up to 15s
	// to finish instead of being cut off mid-request.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("shutdown signal received, draining in-flight requests...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	} else {
		log.Printf("shutdown complete")
	}
}
