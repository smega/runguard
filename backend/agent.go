// agent.go — CLI agent authentication and self-registration.
//
// The runguard CLI authenticates two different ways depending on what it's
// doing:
//   - Sending a heartbeat: no auth at all, just a hard-to-guess key in the
//     URL (/hb/:key) — this is unchanged and predates this file.
//   - Registering a job for the first time (`runguard register`): the CLI
//     doesn't know a heartbeat key yet, so it needs to authenticate as the
//     account owner some other way. That's what AgentToken is for — a
//     long-lived bearer token, separate from the browser session cookie,
//     generated lazily and viewable/regenerable from the account page.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---------- Token issuance (cookie-authenticated, account owner only) ----------

// getOrCreateAgentToken returns the user's existing agent token, generating
// and persisting one on first use. Uses FindOneAndUpdate with $setOnInsert-
// style guarding (a plain conditional set) so concurrent requests can't each
// generate a different token and race each other.
func (a *App) getOrCreateAgentToken(ctx context.Context, u *User) (string, error) {
	if strings.TrimSpace(u.AgentToken) != "" {
		return u.AgentToken, nil
	}

	tok, err := generateTokenString(24)
	if err != nil {
		return "", err
	}
	tok = "rgat_" + tok // "runguard agent token" prefix, makes leaked tokens greppable

	// Only set it if it's still empty — avoids clobbering a token created by
	// a concurrent request between our read above and this write.
	res := a.usersColl.FindOneAndUpdate(
		ctx,
		bson.M{"_id": u.ID, "$or": []bson.M{
			{"agent_token": bson.M{"$exists": false}},
			{"agent_token": ""},
		}},
		bson.M{"$set": bson.M{"agent_token": tok}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var updated User
	if err := res.Decode(&updated); err != nil {
		// Someone else won the race and already set it; re-read.
		var existing User
		if ferr := a.usersColl.FindOne(ctx, bson.M{"_id": u.ID}).Decode(&existing); ferr != nil {
			return "", err
		}
		if existing.AgentToken == "" {
			return "", fmt.Errorf("failed to provision agent token")
		}
		return existing.AgentToken, nil
	}
	return updated.AgentToken, nil
}

func (a *App) handleGetAgentToken(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	tok, err := a.getOrCreateAgentToken(ctx, u)
	if err != nil {
		log.Printf("get/create agent token for %s: %v", u.ID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agent_token": tok})
}

func (a *App) handleRegenerateAgentToken(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	tok, err := generateTokenString(24)
	if err != nil {
		log.Printf("generate agent token for %s: %v", u.ID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	tok = "rgat_" + tok

	if _, err := a.usersColl.UpdateOne(ctx, bson.M{"_id": u.ID}, bson.M{"$set": bson.M{"agent_token": tok}}); err != nil {
		log.Printf("regenerate agent token for %s: %v", u.ID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agent_token": tok})
}

// ---------- Bearer-token auth middleware (CLI, not the browser) ----------

// agentAuthMiddleware authenticates POST /api/agent/* routes via
// "Authorization: Bearer <agent_token>" and sets the same "user" context key
// that authMiddleware sets, so downstream handlers can keep using
// currentUser(c) regardless of which auth path was used.
func (a *App) agentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			return
		}
		tok := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		if tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing agent token"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var u User
		if err := a.usersColl.FindOne(ctx, bson.M{"agent_token": tok}).Decode(&u); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return
		}

		c.Set("user", &u)
		c.Next()
	}
}

// ---------- Self-registration ----------

type agentRegisterRequest struct {
	Name            string `json:"name" binding:"required"`
	ProjectID       string `json:"project_id,omitempty"`
	IntervalMinutes int    `json:"interval_minutes"`
	GraceMinutes    int    `json:"grace_minutes"`
	Severity        string `json:"severity"`
}

type agentRegisterResponse struct {
	JobID        string `json:"job_id"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	HeartbeatURL string `json:"heartbeat_url"`
	Created      bool   `json:"created"`
}

// handleAgentRegister finds-or-creates a job by name for the authenticated
// (via agent token) user, and returns its heartbeat key/URL. Idempotent by
// design: running `runguard register --name=X` twice on the same box should
// hand back the same job rather than creating a duplicate every time cron
// re-runs a setup script.
func (a *App) handleAgentRegister(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req agentRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = 60 // default: expect at least hourly if the caller didn't say
	}
	if req.GraceMinutes < 0 {
		req.GraceMinutes = 0
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// --- Idempotency: return the existing job if one with this name already
	// exists for this user, instead of creating a duplicate. ---
	var existing Job
	err := a.jobsColl.FindOne(ctx, bson.M{
		"user_id":    u.ID,
		"name":       req.Name,
		"deleted_at": bson.M{"$exists": false},
	}).Decode(&existing)
	if err == nil {
		c.JSON(http.StatusOK, agentRegisterResponse{
			JobID:        existing.ID.Hex(),
			Name:         existing.Name,
			Key:          existing.Key,
			HeartbeatURL: fmt.Sprintf("%s/hb/%s", a.cfg.PublicBaseURL, existing.Key),
			Created:      false,
		})
		return
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("find existing job by name for agent register: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// --- PLAN LIMIT ENFORCEMENT (same rule as handleCreateJob) ---
	plan := a.getPlanByID(u.Plan)
	if !u.IsAdmin && plan.MaxJobs > 0 {
		count, cerr := a.jobsColl.CountDocuments(ctx, bson.M{
			"user_id":    u.ID,
			"deleted_at": bson.M{"$exists": false},
		})
		if cerr != nil {
			log.Printf("count jobs for user %s: %v", u.ID.Hex(), cerr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if int(count) >= plan.MaxJobs {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "job limit reached for your plan",
				"max_jobs": plan.MaxJobs,
			})
			return
		}
	}

	// Note: unlike handleCreateJob, self-registration deliberately doesn't
	// accept an arbitrary project_id from the CLI — the job lands unfiled
	// and can be moved into a project from the web UI afterwards.

	token, terr := generateTokenString(16)
	if terr != nil {
		log.Printf("generate job key: %v", terr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	slug := strings.ToUpper(strings.ReplaceAll(req.Name, " ", "_"))
	if len(slug) > 32 {
		slug = slug[:32]
	}
	key := fmt.Sprintf("rg_%s_%s", token, slug)

	now := time.Now().UTC()
	job := Job{
		UserID:               u.ID,
		Name:                 req.Name,
		Key:                  key,
		ExpectedIntervalSecs: req.IntervalMinutes * 60,
		GraceSecs:            req.GraceMinutes * 60,
		CreatedAt:            now,
		UpdatedAt:            now,
		AlertEmailEnabled:    false,
		AlertWebhookEnabled:  true,
		AlertWebhookURLs:     []string{},
		Severity:             normalizeSeverity(req.Severity),
	}

	res, ierr := a.jobsColl.InsertOne(ctx, job)
	if ierr != nil {
		log.Printf("insert job (agent register): %v", ierr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	job.ID = res.InsertedID.(primitive.ObjectID)

	c.JSON(http.StatusCreated, agentRegisterResponse{
		JobID:        job.ID.Hex(),
		Name:         job.Name,
		Key:          job.Key,
		HeartbeatURL: fmt.Sprintf("%s/hb/%s", a.cfg.PublicBaseURL, job.Key),
		Created:      true,
	})
}
