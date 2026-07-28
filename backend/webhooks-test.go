// handler_job_test_webhooks.go
package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (a *App) handleTestJobWebhooks(c *gin.Context) {
	// user from authMiddleware
	uVal, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, ok := uVal.(*User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idStr := c.Param("id")
	jobID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	// Load job
	var job Job
	if err := a.jobsColl.FindOne(c.Request.Context(), bson.M{"_id": jobID}).Decode(&job); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Ownership or admin
	if job.UserID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// Require webhooks to be enabled and at least one URL
	if !job.AlertWebhookEnabled || len(job.AlertWebhookURLs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No enabled webhook URLs for this job.",
		})
		return
	}

	now := time.Now().UTC()
	// ✅ status is computed, not a struct field
	status := computeJobStatus(&job, now)

	payload := map[string]interface{}{
		"text": fmt.Sprintf(
			"RunGuard: test webhook for job '%s' (%s). No action required.",
			job.Name,
			user.Email,
		),
		"event": "job_test",
		"job": map[string]interface{}{
			"id":                job.ID.Hex(),
			"name":              job.Name,
			"status":            status, // ✅ use computed status here
			"severity":          normalizeSeverity(job.Severity),
			"last_heartbeat_at": job.LastHeartbeatAt,
		},
		"user": map[string]interface{}{
			"email": user.Email,
		},
		"sent_at": now.Format(time.RFC3339),
	}

	lastErr := a.sendJobWebhooksWithPayload(c.Request.Context(), user, &job, payload)

	if lastErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":         false,
			"last_error": lastErr.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"last_error": "",
	})
}
