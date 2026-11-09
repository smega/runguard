// webhook_logs.go
package main

import (
	"context"
	"net/http"
	"time"

	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WebhookStatus string

const (
	WebhookStatusSuccess WebhookStatus = "success"
	WebhookStatusFailed  WebhookStatus = "failed"
	WebhookStatusSkipped WebhookStatus = "skipped"
)

// WebhookLog stores a single webhook delivery attempt (with retry support later).
type WebhookLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	JobID          primitive.ObjectID `bson:"job_id" json:"job_id"`
	RunID          primitive.ObjectID `bson:"run_id" json:"run_id"`
	UserID         primitive.ObjectID `bson:"user_id" json:"user_id"`
	URL            string             `bson:"url" json:"url"`
	Status         WebhookStatus      `bson:"status" json:"status"`
	Code           int                `bson:"code"   json:"code"`
	Template       string             `bson:"template" json:"template"` // "json" | "slack" | "custom"
	FirstAttemptAt time.Time          `bson:"first_attempt_at" json:"first_attempt_at"`
	FinalAttemptAt time.Time          `bson:"final_attempt_at" json:"final_attempt_at"`
	AttemptCount   int                `bson:"attempt_count" json:"attempt_count"`
	Attempt        int                `bson:"attempt"      json:"attempt"`
	Success        bool               `bson:"success" json:"success"`
	StatusCode     int                `bson:"status_code,omitempty" json:"status_code,omitempty"`
	DurationMs     int64              `bson:"duration_ms,omitempty" json:"duration_ms,omitempty"`
	Error          string             `bson:"error,omitempty" json:"error,omitempty"`

	MaxAttempts int `bson:"max_attempts" json:"max_attempts"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

// helper to insert a log; ignore the error in the caller if it's best-effort
func (a *App) insertWebhookLog(ctx context.Context, log *WebhookLog) {
	log.CreatedAt = time.Now().UTC()
	_, _ = a.webhookLogsColl.InsertOne(ctx, log)
}

// optional small helper for error truncation, if you want
func truncateErr(msg string, max int) string {
	if len(msg) <= max {
		return msg
	}
	return msg[:max] + "…"
}

type WebhookLogDTO struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	Status      string `json:"status"`
	Code        int    `json:"code"`
	DurationMs  int64  `json:"duration_ms"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	Error       string `json:"error,omitempty"`
	URL         string `json:"url"`
}

func (a *App) handleJobWebhookLogs(c *gin.Context) {
	// user is set by authMiddleware via c.Set("user", *User)
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

	// Fetch job to ensure it belongs to the user (or admin)
	var job Job
	err = a.jobsColl.FindOne(c.Request.Context(), bson.M{"_id": jobID}).Decode(&job)
	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if job.UserID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	limit := int64(20)
	if s := c.Query("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 200 {
			limit = int64(v)
		}
	}

	cur, err := a.webhookLogsColl.Find(
		c.Request.Context(),
		bson.M{
			"job_id":  jobID,
			"user_id": user.ID,
		},
		options.Find().
			SetSort(bson.D{{Key: "created_at", Value: -1}}).
			SetLimit(limit),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer cur.Close(c.Request.Context())

	var logs []WebhookLog
	if err := cur.All(c.Request.Context(), &logs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	out := make([]WebhookLogDTO, 0, len(logs))
	for _, l := range logs {
		out = append(out, WebhookLogDTO{
			ID:          l.ID.Hex(),
			CreatedAt:   l.CreatedAt.Format(time.RFC3339),
			Status:      string(l.Status),
			Code:        l.Code,
			DurationMs:  l.DurationMs,
			Attempt:     l.Attempt,
			MaxAttempts: l.MaxAttempts,
			Error:       l.Error,
			URL:         l.URL, // if you prefer, you can mask to host/path
		})
	}

	c.JSON(http.StatusOK, gin.H{"logs": out})
}

func EnsureJobRunIndexes(ctx context.Context, coll *mongo.Collection) error {
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "job_id", Value: 1}, {Key: "ts", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "ts", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "ts", Value: -1}},
		},
	})
	return err
}

func EnsureWebhookLogIndexes(ctx context.Context, coll *mongo.Collection) error {
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "job_id", Value: 1}, {Key: "run_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "final_attempt_at", Value: -1}},
		},
	})
	return err
}

// Webhook JSON envelope

type WebhookJob struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Env  string   `json:"env,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

type WebhookRun struct {
	ID         string    `json:"id"`
	Ts         time.Time `json:"ts"`
	Status     string    `json:"status"` // ok|fail|late
	ExitCode   *int      `json:"exit_code,omitempty"`
	DurationMs int64     `json:"duration_ms"`
	Attempt    int       `json:"attempt"`
}

type WebhookPayload struct {
	StdoutTail  string     `json:"stdout_tail,omitempty"`
	StderrTail  string     `json:"stderr_tail,omitempty"`
	Uptime30d   *float64   `json:"uptime_30d,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
}

type WebhookRunGuardMeta struct {
	Version    string `json:"version,omitempty"`
	Region     string `json:"region,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
}

type WebhookEnvelope struct {
	Job     WebhookJob          `json:"job"`
	Run     WebhookRun          `json:"run"`
	Payload WebhookPayload      `json:"payload"`
	RG      WebhookRunGuardMeta `json:"runguard"`
}

func safeTail(lt *LogTail) string {
	if lt == nil {
		return ""
	}
	return lt.Tail
}

func BuildWebhookEnvelope(job *Job, run *JobRun, uptime30d *float64, lastSuccess *time.Time) WebhookEnvelope {
	return WebhookEnvelope{
		Job: WebhookJob{
			ID:   job.ID.Hex(),
			Name: job.Name,
			Env:  run.Env,
			Tags: run.Tags,
		},
		Run: WebhookRun{
			ID:         run.ID.Hex(),
			Ts:         run.Ts,
			Status:     run.Status,
			ExitCode:   run.ExitCode,
			DurationMs: run.DurationMs,
			Attempt:    1, // extend when you add retries
		},
		Payload: WebhookPayload{
			StdoutTail:  safeTail(run.StdoutTail),
			StderrTail:  safeTail(run.StderrTail),
			Uptime30d:   uptime30d,
			LastSuccess: lastSuccess,
		},
		RG: WebhookRunGuardMeta{
			Version: run.AgentVersion,
		},
	}
}

type captureLimits struct {
	MaxStdout int
	MaxStderr int
}

// No plan tiers anymore -- everyone gets the same (generous) capture
// limit, matching what the old top tier got.
func defaultCaptureLimits() captureLimits {
	return captureLimits{MaxStdout: 32 * 1024, MaxStderr: 32 * 1024}
}

func makeLogTail(raw string, limit int) *LogTail {
	if limit <= 0 || raw == "" {
		return nil
	}
	if len(raw) <= limit {
		return &LogTail{Tail: raw, Truncated: false}
	}
	return &LogTail{Tail: raw[len(raw)-limit:], Truncated: true}
}

func (a *App) loadUser(ctx context.Context, id primitive.ObjectID) (*User, error) {
	var u User
	if err := a.usersColl.FindOne(ctx, bson.M{"_id": id}).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}
