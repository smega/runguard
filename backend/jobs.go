// jobs.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---------- Requests / responses ----------

type createJobRequest struct {
	Name      string `json:"name" binding:"required"`
	ProjectID string `json:"project_id,omitempty"`
	// IntervalMinutes is required unless CronExpr is set instead (validated
	// in handleCreateJob, since binding tags can't express "one of two
	// fields required").
	IntervalMinutes int    `json:"interval_minutes"`
	GraceMinutes    int    `json:"grace_minutes" binding:"min=0"`
	Severity        string `json:"severity"`
	// CronExpr, if set, switches the job to schedule-based (rather than
	// interval-based) lateness checking. Standard 5-field crontab syntax,
	// evaluated in UTC. See cron.go.
	CronExpr string `json:"cron_expr,omitempty"`
}

type updateJobRequest struct {
	Name                *string  `json:"name,omitempty"`
	ProjectID           *string  `json:"project_id,omitempty"`
	IntervalMinutes     *int     `json:"interval_minutes,omitempty"`
	GraceMinutes        *int     `json:"grace_minutes,omitempty"`
	AlertEmailEnabled   *bool    `json:"alert_email_enabled,omitempty"`
	AlertWebhookEnabled *bool    `json:"alert_webhook_enabled,omitempty"`
	AlertWebhookURLs    []string `json:"alert_webhook_urls,omitempty"`
	PublicEnabled       *bool    `json:"public_enabled,omitempty"`
	Severity            *string  `json:"severity"`
	MaintenanceMinutes  *int64   `json:"maintenance_minutes"`
	MaintenanceUntil    *string  `json:"maintenance_until"` // ISO8601 string or "" to clear
	// MaintenanceMode: nil = leave untouched, "" or "duration" = plain
	// fixed-duration window (driven by MaintenanceMinutes/MaintenanceUntil
	// above), "until_success" = silence until the next successful
	// heartbeat, "manual" = silence indefinitely until explicitly cleared.
	MaintenanceMode *string `json:"maintenance_mode"`
	// CronExpr: nil = leave untouched, "" = clear back to interval mode,
	// non-empty = validate and set as the job's schedule.
	CronExpr *string `json:"cron_expr,omitempty"`
}

const (
	MaintenanceModeDuration     = ""
	MaintenanceModeUntilSuccess = "until_success"
	MaintenanceModeManual       = "manual"
)

func normalizeMaintenanceMode(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "duration":
		return MaintenanceModeDuration, nil
	case MaintenanceModeUntilSuccess:
		return MaintenanceModeUntilSuccess, nil
	case MaintenanceModeManual:
		return MaintenanceModeManual, nil
	default:
		return "", fmt.Errorf("maintenance_mode must be one of: duration, until_success, manual")
	}
}

// jobIsSilenced reports whether alerts should currently be suppressed for
// this job -- true for the two indefinite freeze modes, or for a plain
// fixed-duration window that hasn't expired yet.
func jobIsSilenced(j *Job, now time.Time) bool {
	switch j.MaintenanceMode {
	case MaintenanceModeUntilSuccess, MaintenanceModeManual:
		return true
	}
	return j.MaintenanceUntil != nil && !j.MaintenanceUntil.IsZero() && j.MaintenanceUntil.After(now)
}

type jobResponse struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	ProjectID            string     `json:"project_id,omitempty"`
	ProjectName          string     `json:"project_name,omitempty"`
	Status               string     `json:"status"`
	HeartbeatURL         string     `json:"heartbeat_url"`
	LastHeartbeatAt      *time.Time `json:"last_heartbeat_at,omitempty"`
	ExpectedIntervalSecs int        `json:"expected_interval_seconds"`
	GraceSecs            int        `json:"grace_seconds"`
	CronExpr             string     `json:"cron_expr,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	AlertEmailEnabled   bool       `json:"alert_email_enabled"`
	AlertWebhookEnabled bool       `json:"alert_webhook_enabled"`
	LastAlertSentAt     *time.Time `json:"last_alert_sent_at,omitempty"`
	AlertWebhookURLs    []string   `json:"alert_webhook_urls,omitempty"`

	PublicEnabled bool   `json:"publicEnabled"`
	PublicToken   string `json:"publicToken,omitempty"`

	Severity         string     `json:"severity,omitempty"`
	MaintenanceUntil *time.Time `json:"maintenance_until,omitempty"`
	MaintenanceMode  string     `json:"maintenance_mode,omitempty"`
}

type jobRunResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	At        time.Time `json:"at"`
	LateBySec int64     `json:"late_by_seconds,omitempty"`
}

type jobStatsResponse struct {
	// Mirrors the 7-day window (windows[0]) at the top level so existing
	// callers reading these fields directly keep working unchanged.
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
	Total         int       `json:"total"`
	Ok            int       `json:"ok"`
	Late          int       `json:"late"`
	Fail          int       `json:"fail"`
	UptimePercent float64   `json:"uptime_percent"`

	Windows []uptimeWindow `json:"windows"`
}

type uptimeWindow struct {
	Days          int       `json:"days"`
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
	Total         int       `json:"total"`
	Ok            int       `json:"ok"`
	Late          int       `json:"late"`
	Fail          int       `json:"fail"`
	UptimePercent float64   `json:"uptime_percent"`
}

type publicJobResponse struct {
	Name                 string     `json:"name"`
	Status               string     `json:"status"`
	ExpectedIntervalSecs int        `json:"expected_interval_seconds"`
	GraceSecs            int        `json:"grace_seconds"`
	LastHeartbeatAt      *time.Time `json:"last_heartbeat_at,omitempty"`
	Uptime7d             struct {
		Ok        int     `json:"ok"`
		Late      int     `json:"late"`
		Total     int     `json:"total"`
		PercentOk float64 `json:"percentOk"`
	} `json:"uptime7d"`
}

// ---------- Log tails + webhook logs ----------

type LogTail struct {
	Tail      string `bson:"tail,omitempty" json:"tail,omitempty"`
	Truncated bool   `bson:"truncated,omitempty" json:"truncated,omitempty"`
}

// ---------- Auth helper ----------

func currentUser(c *gin.Context) (*User, bool) {
	u, ok := c.MustGet("user").(*User)
	return u, ok && u != nil
}

// ---------- Status helpers ----------

func projectLabel(ctx context.Context, coll *mongo.Collection, projectID primitive.ObjectID) string {
	if coll == nil || projectID.IsZero() {
		return ""
	}
	var p Project
	if err := coll.FindOne(ctx, bson.M{"_id": projectID, "deleted_at": bson.M{"$exists": false}}).Decode(&p); err != nil {
		return ""
	}
	return p.Name
}

func computeJobStatus(j *Job, now time.Time) string {
	late, _, ok := jobLateness(j, now)
	if !ok {
		return "never_seen"
	}
	if late {
		return "late"
	}
	return "ok"
}

// ---------- Jobs CRUD ----------

func (a *App) handleCreateJob(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.GraceMinutes < 0 {
		req.GraceMinutes = 0
	}

	req.CronExpr = strings.TrimSpace(req.CronExpr)
	if req.CronExpr == "" && req.IntervalMinutes <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "either interval_minutes or cron_expr is required"})
		return
	}
	if req.CronExpr != "" {
		if _, err := ParseCronSchedule(req.CronExpr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid cron_expr: %v", err)})
			return
		}
	}
	if req.IntervalMinutes < 0 {
		req.IntervalMinutes = 0
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// --- PLAN LIMIT ENFORCEMENT ---
	plan := a.getPlanByID(u.Plan)

	// admins: no hard job limit
	if !u.IsAdmin && plan.MaxJobs > 0 {
		count, err := a.jobsColl.CountDocuments(ctx, bson.M{
			"user_id":    u.ID,
			"deleted_at": bson.M{"$exists": false},
		})
		if err != nil {
			log.Printf("count jobs for user %s: %v", u.ID.Hex(), err)
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
	// --- END PLAN LIMIT ---

	var projectID primitive.ObjectID
	if strings.TrimSpace(req.ProjectID) != "" {
		pid, err := primitive.ObjectIDFromHex(strings.TrimSpace(req.ProjectID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
			return
		}
		var proj Project
		if err := a.projectsColl.FindOne(ctx, bson.M{"_id": pid, "user_id": u.ID, "deleted_at": bson.M{"$exists": false}}).Decode(&proj); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project not found"})
			return
		}
		projectID = pid
	}

	token, err := generateTokenString(16)
	if err != nil {
		log.Printf("generate job key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	slug := strings.ToUpper(strings.ReplaceAll(req.Name, " ", "_"))
	if len(slug) > 32 {
		slug = slug[:32]
	}
	key := fmt.Sprintf("rg_%s_%s", token, slug)

	now := time.Now().UTC()
	sev := normalizeSeverity(req.Severity)
	job := Job{
		UserID:               u.ID,
		Name:                 req.Name,
		ProjectID:            projectID,
		Key:                  key,
		ExpectedIntervalSecs: req.IntervalMinutes * 60,
		GraceSecs:            req.GraceMinutes * 60,
		CronExpr:             req.CronExpr,
		CreatedAt:            now,
		UpdatedAt:            now,

		// alert defaults: webhooks on, email off
		AlertEmailEnabled:   false,
		AlertWebhookEnabled: true,
		AlertWebhookURLs:    []string{},

		Severity: sev,
	}

	res, err := a.jobsColl.InsertOne(ctx, job)
	if err != nil {
		log.Printf("insert job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	job.ID = res.InsertedID.(primitive.ObjectID)

	heartbeatURL := fmt.Sprintf("%s/hb/%s", a.cfg.PublicBaseURL, job.Key)

	resp := jobResponse{
		ID:   job.ID.Hex(),
		Name: job.Name,
		ProjectID: func() string {
			if job.ProjectID.IsZero() {
				return ""
			}
			return job.ProjectID.Hex()
		}(),
		ProjectName:          projectLabel(ctx, a.projectsColl, job.ProjectID),
		Status:               computeJobStatus(&job, now),
		HeartbeatURL:         heartbeatURL,
		LastHeartbeatAt:      job.LastHeartbeatAt,
		ExpectedIntervalSecs: job.ExpectedIntervalSecs,
		GraceSecs:            job.GraceSecs,
		CronExpr:             job.CronExpr,
		CreatedAt:            job.CreatedAt,
		UpdatedAt:            job.UpdatedAt,
		AlertEmailEnabled:    job.AlertEmailEnabled,
		AlertWebhookEnabled:  job.AlertWebhookEnabled,
		LastAlertSentAt:      job.LastAlertSentAt,
		AlertWebhookURLs:     job.AlertWebhookURLs,
		Severity:             job.Severity,
		MaintenanceUntil:     job.MaintenanceUntil,
		MaintenanceMode:      job.MaintenanceMode,
	}

	c.JSON(http.StatusCreated, gin.H{"job": resp})
}

func (a *App) handleListJobs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	u := c.MustGet("user").(*User)

	now := time.Now().UTC()

	filter := bson.M{
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}

	cur, err := a.jobsColl.Find(ctx, filter)
	if err != nil {
		log.Printf("list jobs find: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list jobs"})
		return
	}
	defer cur.Close(ctx)

	var jobs []jobResponse

	for cur.Next(ctx) {
		var j Job
		if err := cur.Decode(&j); err != nil {
			log.Printf("list jobs decode: %v", err)
			continue
		}

		status := computeJobStatus(&j, now)
		heartbeatURL := fmt.Sprintf("%s/hb/%s", a.cfg.PublicBaseURL, j.Key)

		jobs = append(jobs, jobResponse{
			ID:   j.ID.Hex(),
			Name: j.Name,
			ProjectID: func() string {
				if j.ProjectID.IsZero() {
					return ""
				}
				return j.ProjectID.Hex()
			}(),
			ProjectName:          projectLabel(ctx, a.projectsColl, j.ProjectID),
			Status:               status,
			HeartbeatURL:         heartbeatURL,
			LastHeartbeatAt:      j.LastHeartbeatAt,
			ExpectedIntervalSecs: j.ExpectedIntervalSecs,
			GraceSecs:            j.GraceSecs,
			CronExpr:             j.CronExpr,
			CreatedAt:            j.CreatedAt,
			UpdatedAt:            j.UpdatedAt,
			AlertEmailEnabled:    j.AlertEmailEnabled,
			AlertWebhookEnabled:  j.AlertWebhookEnabled,
			LastAlertSentAt:      j.LastAlertSentAt,
			AlertWebhookURLs:     j.AlertWebhookURLs,
			PublicEnabled:        j.PublicEnabled,
			PublicToken:          j.PublicToken,
			Severity:             j.Severity,
			MaintenanceUntil:     j.MaintenanceUntil,
			MaintenanceMode:      j.MaintenanceMode,
		})
	}

	if err := cur.Err(); err != nil {
		log.Printf("list jobs cursor err: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs": jobs,
	})
}

func (a *App) handleGetJob(c *gin.Context) {
	u := c.MustGet("user").(*User)
	jobID := c.Param("id")

	id, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var job Job
	if err := a.jobsColl.FindOne(ctx, bson.M{
		"_id":        id,
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}).Decode(&job); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		log.Printf("get job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load job"})
		return
	}

	now := time.Now().UTC()
	status := computeJobStatus(&job, now)
	heartbeatURL := fmt.Sprintf("%s/hb/%s", a.cfg.PublicBaseURL, job.Key)

	resp := jobResponse{
		ID:   job.ID.Hex(),
		Name: job.Name,
		ProjectID: func() string {
			if job.ProjectID.IsZero() {
				return ""
			}
			return job.ProjectID.Hex()
		}(),
		ProjectName:          projectLabel(ctx, a.projectsColl, job.ProjectID),
		Status:               status,
		HeartbeatURL:         heartbeatURL,
		LastHeartbeatAt:      job.LastHeartbeatAt,
		ExpectedIntervalSecs: job.ExpectedIntervalSecs,
		GraceSecs:            job.GraceSecs,
		CronExpr:             job.CronExpr,
		CreatedAt:            job.CreatedAt,
		UpdatedAt:            job.UpdatedAt,
		AlertEmailEnabled:    job.AlertEmailEnabled,
		AlertWebhookEnabled:  job.AlertWebhookEnabled,
		LastAlertSentAt:      job.LastAlertSentAt,
		AlertWebhookURLs:     job.AlertWebhookURLs,
		PublicEnabled:        job.PublicEnabled,
		PublicToken:          job.PublicToken,
		Severity:             job.Severity,
		MaintenanceUntil:     job.MaintenanceUntil,
		MaintenanceMode:      job.MaintenanceMode,
	}

	c.JSON(http.StatusOK, gin.H{"job": resp})
}

func (a *App) handleListJobRuns(c *gin.Context) {
	u := c.MustGet("user").(*User)
	jobID := c.Param("id")

	id, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// ensure job belongs to user
	var job Job
	if err := a.jobsColl.FindOne(ctx, bson.M{
		"_id":        id,
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}).Decode(&job); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		log.Printf("list job runs load job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load job"})
		return
	}

	limit := int64(50)
	if qs := c.Query("limit"); qs != "" {
		if v, err := strconv.Atoi(qs); err == nil && v > 0 && v <= 500 {
			limit = int64(v)
		}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "at", Value: -1}}).
		SetLimit(limit)

	cur, err := a.jobRunsColl.Find(ctx, bson.M{
		"job_id":  job.ID,
		"user_id": u.ID,
	}, opts)
	if err != nil {
		log.Printf("list job runs find: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list runs"})
		return
	}
	defer cur.Close(ctx)

	var runs []jobRunResponse
	for cur.Next(ctx) {
		var r JobRun
		if err := cur.Decode(&r); err != nil {
			log.Printf("list job runs decode: %v", err)
			continue
		}
		runs = append(runs, jobRunResponse{
			ID:        r.ID.Hex(),
			Status:    r.Status,
			At:        r.At,
			LateBySec: r.LateBySec,
		})
	}
	if err := cur.Err(); err != nil {
		log.Printf("list job runs cursor: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

// PATCH /api/jobs/:id
func (a *App) handleUpdateJob(c *gin.Context) {
	jobID := c.Param("id")
	id, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	var req updateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	u := c.MustGet("user").(*User)
	plan := a.getUserPlanFeatures(u)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var job Job
	if err := a.jobsColl.FindOne(ctx, bson.M{
		"_id":        id,
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}).Decode(&job); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		log.Printf("update job find: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load job"})
		return
	}

	update := bson.M{}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		update["name"] = strings.TrimSpace(*req.Name)
	}
	if req.ProjectID != nil {
		p := strings.TrimSpace(*req.ProjectID)
		if p == "" {
			update["project_id"] = primitive.NilObjectID
		} else {
			pid, err := primitive.ObjectIDFromHex(p)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
				return
			}
			var proj Project
			if err := a.projectsColl.FindOne(ctx, bson.M{"_id": pid, "user_id": u.ID, "deleted_at": bson.M{"$exists": false}}).Decode(&proj); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "project not found"})
				return
			}
			update["project_id"] = pid
		}
	}
	if req.IntervalMinutes != nil && *req.IntervalMinutes > 0 {
		update["expected_interval_seconds"] = (*req.IntervalMinutes) * 60
	}
	if req.GraceMinutes != nil && *req.GraceMinutes >= 0 {
		update["grace_seconds"] = (*req.GraceMinutes) * 60
	}
	if req.AlertEmailEnabled != nil {
		if *req.AlertEmailEnabled && !plan.AllowJobEmailAlerts {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "email alerts are not available on your plan",
			})
			return
		}
		update["alert_email_enabled"] = *req.AlertEmailEnabled
	}
	if req.AlertWebhookEnabled != nil {
		update["alert_webhook_enabled"] = *req.AlertWebhookEnabled
	}
	if req.AlertWebhookURLs != nil {
		urls := req.AlertWebhookURLs

		// basic sanitise + de-duplicate
		seen := make(map[string]bool)
		cleaned := make([]string, 0, len(urls))
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			if err := validateWebhookURLScheme(u); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if !seen[u] {
				seen[u] = true
				cleaned = append(cleaned, u)
			}
		}
		update["alert_webhook_urls"] = cleaned
	}
	if req.PublicEnabled != nil {
		if *req.PublicEnabled && !plan.AllowPublicLinks {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "public status links are not available on your plan",
			})
			return
		}
		update["publicEnabled"] = *req.PublicEnabled

		// If turning public on and no token yet, generate one
		if *req.PublicEnabled && !job.PublicEnabled {
			if job.PublicToken == "" {
				token, err := generateTokenString(16)
				if err != nil {
					log.Printf("generate public token: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
					return
				}
				update["publicToken"] = token
			}
			update["publicEnabled"] = true
		} else if !*req.PublicEnabled && job.PublicEnabled {
			update["publicEnabled"] = false
		}
	}
	if req.Severity != nil {
		update["severity"] = normalizeSeverity(*req.Severity)
	}
	// Maintenance window (silence alerts)
	now := time.Now().UTC()
	if req.MaintenanceUntil != nil {
		s := strings.TrimSpace(*req.MaintenanceUntil)
		if s == "" {
			update["maintenance_until"] = nil
		} else {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid maintenance_until"})
				return
			}
			update["maintenance_until"] = t
		}
	}
	if req.MaintenanceMinutes != nil {
		m := *req.MaintenanceMinutes
		if m <= 0 {
			zero := time.Time{}
			update["maintenance_until"] = zero
		} else {
			until := now.Add(time.Duration(m) * time.Minute)
			update["maintenance_until"] = until
		}
	}
	if req.MaintenanceMode != nil {
		mode, err := normalizeMaintenanceMode(*req.MaintenanceMode)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		update["maintenance_mode"] = mode
		// Switching into one of the indefinite modes makes any fixed
		// MaintenanceUntil timestamp irrelevant (jobIsSilenced treats the
		// mode as authoritative either way), but clear it too so the
		// frontend doesn't show a stale "until <date>" alongside it. If the
		// request also set MaintenanceMinutes/MaintenanceUntil explicitly in
		// the same call, that still wins since it's applied above.
		if mode != MaintenanceModeDuration {
			if _, explicit := update["maintenance_until"]; !explicit {
				update["maintenance_until"] = nil
			}
		}
	}
	if req.CronExpr != nil {
		expr := strings.TrimSpace(*req.CronExpr)
		if expr == "" {
			update["cron_expr"] = ""
		} else {
			if _, err := ParseCronSchedule(expr); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid cron_expr: %v", err)})
				return
			}
			update["cron_expr"] = expr
		}
	}

	if len(update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}

	update["updated_at"] = now

	_, err = a.jobsColl.UpdateByID(ctx, job.ID, bson.M{"$set": update})
	if err != nil {
		log.Printf("update job UpdateByID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update job"})
		return
	}

	// reload job to return fresh
	if err := a.jobsColl.FindOne(ctx, bson.M{"_id": job.ID}).Decode(&job); err != nil {
		log.Printf("update job reload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload job"})
		return
	}

	status := computeJobStatus(&job, now)
	heartbeatURL := fmt.Sprintf("%s/hb/%s", a.cfg.PublicBaseURL, job.Key)

	resp := jobResponse{
		ID:                   job.ID.Hex(),
		Name:                 job.Name,
		Status:               status,
		HeartbeatURL:         heartbeatURL,
		LastHeartbeatAt:      job.LastHeartbeatAt,
		ExpectedIntervalSecs: job.ExpectedIntervalSecs,
		GraceSecs:            job.GraceSecs,
		CronExpr:             job.CronExpr,
		CreatedAt:            job.CreatedAt,
		UpdatedAt:            job.UpdatedAt,
		AlertEmailEnabled:    job.AlertEmailEnabled,
		AlertWebhookEnabled:  job.AlertWebhookEnabled,
		LastAlertSentAt:      job.LastAlertSentAt,
		AlertWebhookURLs:     job.AlertWebhookURLs,
		PublicEnabled:        job.PublicEnabled,
		PublicToken:          job.PublicToken,
		Severity:             job.Severity,
		MaintenanceUntil:     job.MaintenanceUntil,
		MaintenanceMode:      job.MaintenanceMode,
	}

	c.JSON(http.StatusOK, gin.H{"job": resp})
}

// DELETE /api/jobs/:id (soft delete)
func (a *App) handleDeleteJob(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idStr := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"_id":        objID,
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}

	now := time.Now().UTC()

	res, err := a.jobsColl.UpdateOne(ctx, filter, bson.M{
		"$set": bson.M{"deleted_at": now, "updated_at": now},
	})
	if err != nil {
		log.Printf("soft delete job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------- Heartbeat ----------

// optional payload from the wrapper – all fields are optional
type HeartbeatPayload struct {
	Status       string     `json:"status"`      // ok|fail|late
	ExitCode     *int       `json:"exit_code"`   // optional
	DurationMs   *int64     `json:"duration_ms"` // optional
	Env          string     `json:"env,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	AgentVersion string     `json:"agent_version,omitempty"`
	StdoutTail   string     `json:"stdout_tail,omitempty"`
	StderrTail   string     `json:"stderr_tail,omitempty"`
	Ts           *time.Time `json:"ts,omitempty"` // agent timestamp, optional
}

func (a *App) handleHeartbeat(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var job Job
	err := a.jobsColl.FindOne(ctx, bson.M{
		"key":        key,
		"deleted_at": bson.M{"$exists": false},
	}).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		// job deleted or unknown key
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("find job by key: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// optional JSON payload from wrapper
	var payload HeartbeatPayload
	if c.Request.Body != nil {
		bodyBytes, readErr := io.ReadAll(c.Request.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &payload); err != nil {
				log.Printf("heartbeat: ignoring invalid JSON payload for key=%s: %v", key, err)
			}
		}
	}

	now := time.Now().UTC()

	// derive run attributes
	ts := now
	if payload.Ts != nil {
		ts = payload.Ts.UTC()
	}

	status := payload.Status
	if status == "" {
		status = "ok"
	}

	setFields := bson.M{
		"last_heartbeat_at": now,
		"updated_at":        now,
	}
	unsetFields := bson.M{
		"last_alert_sent_at": "",
	}

	// A job frozen "until first success" exists precisely so nobody has to
	// remember to manually unmute it once the underlying problem is fixed --
	// the first "ok" heartbeat after the freeze clears it automatically.
	if status == "ok" && job.MaintenanceMode == MaintenanceModeUntilSuccess {
		unsetFields["maintenance_mode"] = ""
		unsetFields["maintenance_until"] = ""
	}

	_, err = a.jobsColl.UpdateByID(ctx, job.ID, bson.M{
		"$set":   setFields,
		"$unset": unsetFields,
	})
	if err != nil {
		log.Printf("update last heartbeat: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	exitCode := 0
	if payload.ExitCode != nil {
		exitCode = *payload.ExitCode
	}

	var durationMs int64
	if payload.DurationMs != nil {
		durationMs = *payload.DurationMs
	}

	lateBy := time.Duration(0) // old behavior: heartbeat records "0 late"

	if err := a.insertJobRunWithExtras(ctx, &job, status, ts, lateBy, exitCode, durationMs, &payload); err != nil {
		log.Printf("insert job run (extras): %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	if status == "ok" {
		_ = a.resolveOpenIncidents(ctx, &job, now)
	} else if status == "fail" {
		_ = a.openOrRefreshIncident(ctx, &job, IncidentKindFail, "Run reported failure via heartbeat payload.", status, now)
	}

	c.Status(http.StatusNoContent)
}

// Extended insertion that preserves old behavior and adds metadata/tails.
func (a *App) insertJobRunWithExtras(
	ctx context.Context,
	job *Job,
	status string,
	ts time.Time,
	lateBy time.Duration,
	exitCode int,
	durationMs int64,
	payload *HeartbeatPayload,
) error {
	// 1) existing behavior: write basic JobRun with lateBy
	a.insertJobRun(ctx, job, status, ts, lateBy)

	// 2) no payload => nothing else
	if payload == nil {
		return nil
	}

	// 3) plan-based limits
	limits := captureLimitsFor("free", false)
	if a.usersColl != nil {
		var u User
		if err := a.usersColl.FindOne(ctx, bson.M{"_id": job.UserID}).Decode(&u); err == nil {
			limits = captureLimitsFor(u.Plan, u.IsAdmin)
		} else {
			log.Printf("insertJobRunWithExtras: load user failed, treating as free: %v", err)
		}
	}

	stdoutTail := makeLogTail(payload.StdoutTail, limits.MaxStdout)
	stderrTail := makeLogTail(payload.StderrTail, limits.MaxStderr)

	// 4) patch the basic run with metadata instead of inserting a duplicate row
	filter := bson.M{
		"job_id":  job.ID,
		"user_id": job.UserID,
		"at":      ts,
		"status":  status,
	}

	update := bson.M{
		"$set": bson.M{
			"duration_ms":   durationMs,
			"env":           payload.Env,
			"tags":          payload.Tags,
			"agent_version": payload.AgentVersion,
			"stdout_tail":   stdoutTail,
			"stderr_tail":   stderrTail,
			"exit_code":     exitCode,
		},
	}

	opts := options.Update().SetUpsert(true)

	if _, err := a.jobRunsColl.UpdateOne(ctx, filter, update, opts); err != nil {
		return err
	}

	return nil
}

// Existing helper: keep as-is
func (a *App) insertJobRun(ctx context.Context, job *Job, status string, at time.Time, lateBy time.Duration) {
	run := JobRun{
		JobID:     job.ID,
		UserID:    job.UserID,
		Status:    status,
		At:        at,
		LateBySec: int64(lateBy.Seconds()),
		CreatedAt: time.Now().UTC(),
	}

	if _, err := a.jobRunsColl.InsertOne(ctx, run); err != nil {
		log.Printf("insert job_run (job %s, status=%s): %v", job.ID.Hex(), status, err)
	}
}

// ---------- Stats / uptime ----------

func (a *App) handleJobStats(c *gin.Context) {
	u := c.MustGet("user").(*User)
	jobID := c.Param("id")

	id, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	// A short-lived, dedicated context just for the job lookup -- kept
	// separate from the window queries below so a slow/flaky Mongo
	// connection during the lookup can't eat into their budget too.
	lookupCtx, lookupCancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer lookupCancel()

	// ensure job belongs to this user
	var job Job
	if err := a.jobsColl.FindOne(lookupCtx, bson.M{
		"_id":        id,
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}).Decode(&job); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		log.Printf("job stats load job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load job"})
		return
	}

	now := time.Now().UTC()

	// The 7/30/90-day windows are independent queries against the same
	// collection. Previously they ran sequentially inside one shared
	// 10-second context, so a single slow/flaky connection to Mongo (a
	// transient network blip, not necessarily a slow query) could burn
	// through the whole budget on the first window and starve the rest,
	// turning one hiccup into a full 500 for the endpoint. Now each window
	// gets its own timeout and runs concurrently, and a single window
	// failing no longer takes the other two down with it.
	daysList := []int{7, 30, 90}
	type windowOutcome struct {
		w   uptimeWindow
		err error
	}
	outcomes := make([]windowOutcome, len(daysList))

	var wg sync.WaitGroup
	for i, days := range daysList {
		wg.Add(1)
		go func(i, days int) {
			defer wg.Done()
			wctx, wcancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
			defer wcancel()

			from := now.Add(-time.Duration(days) * 24 * time.Hour)
			w, err := a.computeUptimeWindow(wctx, job.ID, u.ID, days, from, now)
			outcomes[i] = windowOutcome{w: w, err: err}
		}(i, days)
	}
	wg.Wait()

	var windows []uptimeWindow
	var primary *uptimeWindow
	for i, o := range outcomes {
		if o.err != nil {
			log.Printf("job stats window %dd: %v", daysList[i], o.err)
			continue
		}
		windows = append(windows, o.w)
		if daysList[i] == 7 {
			w := o.w
			primary = &w
		}
	}

	if len(windows) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
	}

	// windows[...] mirrors whichever window covers 7 days at the top level
	// too, so existing callers reading stats.total/ok/late/uptime_percent
	// directly don't break. If the 7-day window itself is the one that
	// failed, fall back to the first window that did succeed rather than
	// leaving these fields zeroed out.
	if primary == nil {
		primary = &windows[0]
	}
	resp := jobStatsResponse{
		From:          primary.From,
		To:            primary.To,
		Total:         primary.Total,
		Ok:            primary.Ok,
		Late:          primary.Late,
		Fail:          primary.Fail,
		UptimePercent: primary.UptimePercent,
		Windows:       windows,
	}

	c.JSON(http.StatusOK, gin.H{"stats": resp})
}

// computeUptimeWindow tallies job_run outcomes in [from, to] and computes an
// uptime percentage. "fail" (an explicit non-zero-exit heartbeat reported by
// the CLI) counts as downtime exactly like a missed/late heartbeat -- this
// used to only weigh ok vs late, so a job that failed every single run
// without ever technically being *late* showed 100% uptime, which was wrong.
func (a *App) computeUptimeWindow(ctx context.Context, jobID, userID primitive.ObjectID, days int, from, to time.Time) (uptimeWindow, error) {
	cur, err := a.jobRunsColl.Find(ctx, bson.M{
		"job_id":  jobID,
		"user_id": userID,
		"at": bson.M{
			"$gte": from,
			"$lte": to,
		},
	})
	if err != nil {
		return uptimeWindow{}, fmt.Errorf("find runs: %w", err)
	}
	defer cur.Close(ctx)

	var total, okCount, lateCount, failCount int
	for cur.Next(ctx) {
		var r JobRun
		if err := cur.Decode(&r); err != nil {
			log.Printf("uptime window decode run: %v", err)
			continue
		}
		total++
		switch r.Status {
		case "ok":
			okCount++
		case "late":
			lateCount++
		case "fail":
			failCount++
		}
	}
	if err := cur.Err(); err != nil {
		log.Printf("uptime window cursor: %v", err)
	}

	denom := okCount + lateCount + failCount
	uptime := 0.0
	if denom > 0 {
		uptime = float64(okCount) / float64(denom) * 100.0
	}

	return uptimeWindow{
		Days:          days,
		From:          from,
		To:            to,
		Total:         total,
		Ok:            okCount,
		Late:          lateCount,
		Fail:          failCount,
		UptimePercent: uptime,
	}, nil
}

// ---------- Public job endpoint ----------

// GET /api/public/job/:token
func (a *App) handlePublicJob(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var job Job
	if err := a.jobsColl.FindOne(ctx, bson.M{
		"publicEnabled": true,
		"publicToken":   token,
		"deleted_at":    bson.M{"$exists": false},
	}).Decode(&job); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		log.Printf("public job load: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load job"})
		return
	}

	now := time.Now().UTC()
	from := now.Add(-7 * 24 * time.Hour)

	// job.UserID is used here (rather than a request-scoped user, since
	// this is an unauthenticated public endpoint) purely to scope the
	// job_runs query the same way the authenticated stats endpoint does.
	w, err := a.computeUptimeWindow(ctx, job.ID, job.UserID, 7, from, now)
	if err != nil {
		log.Printf("public job stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
	}

	var resp publicJobResponse
	resp.Name = job.Name
	resp.Status = computeJobStatus(&job, now)
	resp.ExpectedIntervalSecs = job.ExpectedIntervalSecs
	resp.GraceSecs = job.GraceSecs
	resp.LastHeartbeatAt = job.LastHeartbeatAt
	resp.Uptime7d.Ok = w.Ok
	resp.Uptime7d.Late = w.Late
	resp.Uptime7d.Total = w.Total
	resp.Uptime7d.PercentOk = w.UptimePercent

	c.JSON(http.StatusOK, resp)
}

// ---------- Webhook logs API ----------

func (a *App) ListWebhookLogsForRun(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	jobIDHex := c.Param("id")
	runIDHex := c.Param("runID")

	jobID, err := primitive.ObjectIDFromHex(jobIDHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	runID, err := primitive.ObjectIDFromHex(runIDHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := a.webhookLogsColl.Find(
		ctx,
		bson.M{
			"job_id":  jobID,
			"run_id":  runID,
			"user_id": u.ID,
		},
		options.Find().SetSort(bson.D{{Key: "final_attempt_at", Value: -1}}),
	)
	if err != nil {
		log.Printf("list webhook logs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer cur.Close(ctx)

	var logs []WebhookLog
	if err := cur.All(ctx, &logs); err != nil {
		log.Printf("list webhook logs cursor: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, logs)
}
