// app_public.go (part 1)
package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type toggleJobPublicRequest struct {
	Enabled bool `json:"enabled"`
}

type toggleJobPublicResponse struct {
	Enabled   bool   `json:"enabled"`
	PublicURL string `json:"publicUrl,omitempty"`
}

func randomPublicToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return enc.EncodeToString(b), nil
}

func (a *App) handleToggleJobPublic(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	jobIDHex := c.Param("id")
	jobID, err := primitive.ObjectIDFromHex(jobIDHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	var req toggleJobPublicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var job Job
	if err := a.jobsColl.FindOne(ctx, bson.M{
		"_id":        jobID,
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}).Decode(&job); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	update := bson.M{
		"publicEnabled": req.Enabled,
	}

	token := job.PublicToken
	if req.Enabled {
		if token == "" {
			token, err = randomPublicToken()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
				return
			}
			update["publicToken"] = token
		}
	}

	if _, err := a.jobsColl.UpdateByID(ctx, job.ID, bson.M{"$set": update}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update job"})
		return
	}

	resp := toggleJobPublicResponse{
		Enabled: req.Enabled,
	}
	if req.Enabled {
		resp.PublicURL = a.cfg.PublicBaseURL + "/p/" + token
	}

	c.JSON(http.StatusOK, resp)
}

// app_public.go (part 2)
type PublicJobUptime7d struct {
	Ok        int64   `json:"ok"`
	Late      int64   `json:"late"`
	Total     int64   `json:"total"`
	PercentOk float64 `json:"percentOk"`
}

type PublicJobPayload struct {
	Name                 string            `json:"name"`
	Status               string            `json:"status"`
	ExpectedIntervalSecs int               `json:"expected_interval_seconds"`
	GraceSecs            int               `json:"grace_seconds"`
	LastHeartbeatAt      *time.Time        `json:"last_heartbeat_at,omitempty"`
	Uptime7d             PublicJobUptime7d `json:"uptime7d"`
}

// func (a *App) handlePublicJob(c *gin.Context) {
// 	token := c.Param("token")
// 	if token == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
// 	defer cancel()

// 	var job Job
// 	if err := a.jobsColl.FindOne(ctx, bson.M{
// 		"publicToken":   token,
// 		"publicEnabled": true,
// 		"deleted_at":    bson.M{"$exists": false},
// 	}).Decode(&job); err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
// 		return
// 	}

// 	now := time.Now().UTC()
// 	status := computeJobStatus(&job, now)

// 	// Compute basic 7-day uptime from job_runs
// 	from := now.Add(-7 * 24 * time.Hour)

// 	cur, err := a.jobRunsColl.Find(ctx, bson.M{
// 		"job_id":  job.ID,
// 		"user_id": job.UserID,
// 		"at": bson.M{
// 			"$gte": from,
// 			"$lte": now,
// 		},
// 	})
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load runs"})
// 		return
// 	}
// 	defer cur.Close(ctx)

// 	var okCount, lateCount, total int64
// 	for cur.Next(ctx) {
// 		var r JobRun
// 		if err := cur.Decode(&r); err != nil {
// 			continue
// 		}
// 		total++
// 		switch r.Status {
// 		case "ok":
// 			okCount++
// 		case "late":
// 			lateCount++
// 		}
// 	}
// 	_ = cur.Err() // ignored but you can log

// 	var percentOk float64
// 	if okCount+lateCount > 0 {
// 		percentOk = float64(okCount) / float64(okCount+lateCount) * 100
// 	}

// 	payload := PublicJobPayload{
// 		Name:                 job.Name,
// 		Status:               status,
// 		ExpectedIntervalSecs: job.ExpectedIntervalSecs,
// 		GraceSecs:            job.GraceSecs,
// 		LastHeartbeatAt:      job.LastHeartbeatAt,
// 		Uptime7d: PublicJobUptime7d{
// 			Ok:        okCount,
// 			Late:      lateCount,
// 			Total:     total,
// 			PercentOk: percentOk,
// 		},
// 	}

// 	c.JSON(http.StatusOK, payload)
// }
