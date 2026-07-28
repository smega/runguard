// main.go
package main

import (
	"context"
	"log"
	"net/http"
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

	r := gin.Default()

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

	// Public heartbeat endpoint (no auth)
	r.GET("/hb/:key", app.handleHeartbeat)
	r.POST("/hb/:key", app.handleHeartbeat)

	api := r.Group("/api")

	// Auth routes
	auth := api.Group("/auth")
	{
		auth.POST("/signup", app.handleSignup)
		auth.POST("/login", app.handleLogin)
		auth.POST("/logout", app.handleLogout)
		auth.GET("/me", app.authMiddleware(), app.handleMe)

		auth.POST("/magic/request", app.handleMagicRequest)
		auth.POST("/magic/consume", app.handleMagicConsume)

		auth.POST("/password/reset/request", app.handlePasswordResetRequest)
		auth.POST("/password/reset/confirm", app.handlePasswordResetConfirm)
	}

	account := api.Group("/account").Use(app.authMiddleware())
	{
		account.GET("", app.handleGetAccount)
		account.PATCH("", app.handleUpdateAccount)
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
	}

	log.Printf("listening on %s", cfg.Server.Addr)
	if err := r.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
