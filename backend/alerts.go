// alerts.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (a *App) RunAlertLoop() {
	if !a.cfg.Alerts.Enabled {
		log.Printf("[alerts] disabled; alert loop not started")
		return
	}
	interval := time.Duration(a.cfg.Alerts.CheckIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	log.Printf("[alerts] starting alert loop (interval=%s)", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := a.scanAndAlertLateJobs(ctx); err != nil {
			log.Printf("[alerts] scanAndAlertLateJobs error: %v", err)
		}
		cancel()
	}
}

// scanAndAlertLateJobs finds jobs that are late and sends alerts respecting cooldown.
func (a *App) scanAndAlertLateJobs(ctx context.Context) error {
	now := time.Now().UTC()
	minLate := time.Duration(a.cfg.Alerts.MinLateSeconds) * time.Second
	if minLate < 0 {
		minLate = 0
	}
	cooldown := time.Duration(a.cfg.Alerts.CooldownSeconds) * time.Second
	if cooldown < 0 {
		cooldown = 0
	}

	filter := bson.M{
		"deleted_at": bson.M{"$exists": false},
	}

	cur, err := a.jobsColl.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("find jobs for alerts: %w", err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var j Job
		if err := cur.Decode(&j); err != nil {
			log.Printf("[alerts] decode job: %v", err)
			continue
		}

		// NEW: suppress alerts during maintenance window
		if j.MaintenanceUntil != nil && !j.MaintenanceUntil.IsZero() && j.MaintenanceUntil.After(now) {
			log.Printf(
				"[alerts] job %s (%s) is in maintenance until %s; skipping alerts",
				j.ID.Hex(),
				j.Name,
				j.MaintenanceUntil.UTC().Format(time.RFC3339),
			)
			continue
		}

		if j.LastHeartbeatAt == nil || j.LastHeartbeatAt.IsZero() {
			continue
		}

		// Determine channels for this job
		hasEmail := a.cfg.Mailgun.Enabled && j.AlertEmailEnabled
		hasWebhooks := j.AlertWebhookEnabled && len(j.AlertWebhookURLs) > 0
		if !hasEmail && !hasWebhooks {
			continue
		}

		// jobLateness handles both plain interval+grace jobs and
		// cron-scheduled jobs (Job.CronExpr) uniformly -- see cron.go.
		late, lateBy, seen := jobLateness(&j, now)
		if !seen || !late {
			continue
		}
		status := "late"

		if lateBy < minLate {
			log.Printf(
				"[alerts] job %s (%s) late but below threshold (lateBy=%s, minLate=%s)",
				j.ID.Hex(), j.Name, lateBy, minLate,
			)
			continue
		}

		// Cooldown
		if j.LastAlertSentAt != nil {
			since := now.Sub(*j.LastAlertSentAt)
			if since < cooldown {
				log.Printf(
					"[alerts] job %s (%s) already alerted %s ago (< cooldown %s)",
					j.ID.Hex(), j.Name, since, cooldown,
				)
				continue
			}
		}

		// record a late run snapshot
		a.insertJobRun(ctx, &j, "late", now, lateBy)
		if err := a.openOrRefreshIncident(ctx, &j, IncidentKindLate, "Job missed its expected heartbeat window.", "late", now); err != nil {
			log.Printf("[alerts] open incident for job %s: %v", j.ID.Hex(), err)
		}

		log.Printf(
			"[alerts] job %s (%s) is alertable; late by %s",
			j.ID.Hex(), j.Name, lateBy,
		)

		var u User
		if err := a.usersColl.FindOne(ctx, bson.M{"_id": j.UserID}).Decode(&u); err != nil {
			log.Printf("[alerts] find user for job %s: %v", j.ID.Hex(), err)
			continue
		}

		// Send channels
		if hasEmail {
			if err := a.sendJobLateEmail(&u, &j, now); err != nil {
				log.Printf("[alerts] send email for job %s: %v", j.ID.Hex(), err)
			}
		}
		if hasWebhooks {
			if err := a.sendJobLateWebhooks(ctx, &u, &j, now, status); err != nil {
				log.Printf("[alerts] send webhooks for job %s: %v", j.ID.Hex(), err)
			}
		}

		// Record alert time
		_, err := a.jobsColl.UpdateByID(ctx, j.ID, bson.M{
			"$set": bson.M{
				"last_alert_sent_at": now,
				"updated_at":         now,
			},
		})
		if err != nil {
			log.Printf("[alerts] update last_alert_sent_at for job %s: %v", j.ID.Hex(), err)
		}
	}

	if err := cur.Err(); err != nil {
		return fmt.Errorf("iterate jobs for alerts: %w", err)
	}
	return nil
}

func (a *App) sendJobLateEmail(u *User, j *Job, now time.Time) error {
	subject := fmt.Sprintf("[RunGuard] Job late: %s", j.Name)
	bodyText := fmt.Sprintf(
		"RunGuard has detected that a monitored job is late.\n\n"+
			"Job: %s\n"+
			"Expected interval: ~%d minutes\n"+
			"Grace: %d minutes\n"+
			"Last heartbeat: %s (UTC)\n\n"+
			"You may want to check the underlying cron/systemd/CI job.\n",
		j.Name,
		j.ExpectedIntervalSecs/60,
		j.GraceSecs/60,
		j.LastHeartbeatAt.UTC().Format(time.RFC3339),
	)

	log.Printf("[alerts] sending late-job email to %s for job %s", u.Email, j.Name)
	return a.sendMail(u.Email, subject, bodyText, "")
}

// sendJobLateWebhooks now:
// - uses ctx from the alert loop
// - logs each attempt into webhook_logs via a.insertWebhookLog
func (a *App) sendJobLateWebhooks(ctx context.Context, u *User, j *Job, now time.Time, status string) error {
	if len(j.AlertWebhookURLs) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"text": fmt.Sprintf(
			"RunGuard: job '%s' for %s is late. Last heartbeat: %s UTC.",
			j.Name,
			u.Email,
			j.LastHeartbeatAt.UTC().Format(time.RFC3339),
		),
		"event": "job_late",
		"job": map[string]interface{}{
			"id":                j.ID.Hex(),
			"name":              j.Name,
			"status":            status,
			"last_heartbeat_at": j.LastHeartbeatAt,
			"interval_seconds":  j.ExpectedIntervalSecs,
			"grace_seconds":     j.GraceSecs,
		},
		"user": map[string]interface{}{
			"email": u.Email,
		},
		"sent_at": now.UTC().Format(time.RFC3339),
	}

	return a.sendJobWebhooksWithPayload(ctx, u, j, payload)
}
func (a *App) sendJobWebhooksWithPayload(
	ctx context.Context,
	u *User,
	j *Job,
	payload map[string]interface{},
) error {
	if len(j.AlertWebhookURLs) == 0 {
		return nil
	}

	var lastErr error

	for _, rawURL := range j.AlertWebhookURLs {
		urlStr := strings.TrimSpace(rawURL)
		if urlStr == "" {
			continue
		}

		// Determine template type: default to generic
		tType := WebhookTemplateTypeGeneric
		if a.webhookTemplatesColl != nil {
			if tmpl, err := a.findWebhookTemplateByURL(ctx, j.UserID, urlStr); err == nil && tmpl != nil {
				if tmpl.Type != "" {
					tType = tmpl.Type
				}
			}
		}

		// Build body based on template type (generic JSON vs Slack)
		bodyBytes, err := buildWebhookBodyForType(tType, payload)
		if err != nil {
			lastErr = err

			a.insertWebhookLog(ctx, &WebhookLog{
				JobID:      j.ID,
				URL:        urlStr,
				Status:     "failed",
				Code:       0,
				DurationMs: 0,
				Error:      err.Error(),
			})

			continue
		}

		start := time.Now()
		code, postErr := postJSONWithStatus(ctx, urlStr, bodyBytes)
		durationMs := time.Since(start).Milliseconds()

		if postErr != nil {
			lastErr = postErr

			a.insertWebhookLog(ctx, &WebhookLog{
				JobID:      j.ID,
				URL:        urlStr,
				Status:     "failed",
				Code:       code,
				DurationMs: durationMs,
				Error:      postErr.Error(),
			})
		} else {
			a.insertWebhookLog(ctx, &WebhookLog{
				JobID:      j.ID,
				URL:        urlStr,
				Status:     "success",
				Code:       code,
				DurationMs: durationMs,
				Error:      "",
			})
		}
	}

	return lastErr
}


func buildWebhookBodyForType(tType string, payload map[string]interface{}) ([]byte, error) {
	switch strings.ToLower(tType) {
	case WebhookTemplateTypeSlack:
		// For Slack: focus on "text"
		var text string
		if v, ok := payload["text"].(string); ok && v != "" {
			text = v
		} else {
			// fallback: short JSON dump
			b, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			text = string(b)
		}
		body := map[string]interface{}{
			"text": text,
		}
		return json.Marshal(body)

	case WebhookTemplateTypeDiscord:
		// Discord's incoming webhooks expect {"content": "..."} rather than
		// Slack's {"text": "..."} -- same idea, different field name.
		var content string
		if v, ok := payload["text"].(string); ok && v != "" {
			content = v
		} else {
			b, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			content = string(b)
		}
		// Discord rejects messages over 2000 characters outright, so trim
		// rather than let the whole delivery fail on a long JSON fallback.
		if len(content) > 2000 {
			content = content[:1997] + "..."
		}
		body := map[string]interface{}{
			"content": content,
		}
		return json.Marshal(body)

	default: // generic JSON
		return json.Marshal(payload)
	}
}

// webhookRetryDelays defines the backoff between delivery attempts. A
// transient failure on the receiving end (a blip, a brief 500, a timeout)
// used to just drop the alert after a single try; now it gets a few more
// chances before giving up.
var webhookRetryDelays = []time.Duration{2 * time.Second, 10 * time.Second}

func postJSONWithStatus(ctx context.Context, rawURL string, body []byte) (int, error) {
	if err := validateWebhookURLScheme(rawURL); err != nil {
		return 0, fmt.Errorf("invalid webhook url: %w", err)
	}

	var lastCode int
	var lastErr error

	attempts := len(webhookRetryDelays) + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return lastCode, ctx.Err()
			case <-time.After(webhookRetryDelays[attempt-1]):
			}
		}

		code, err := postJSONOnce(ctx, rawURL, body)
		if err == nil {
			return code, nil
		}
		lastCode, lastErr = code, err

		// Don't retry on responses that mean "this will never succeed"
		// (4xx other than 429) — only retry timeouts/network errors/5xx/429.
		if code >= 400 && code < 500 && code != http.StatusTooManyRequests {
			return code, err
		}
	}
	return lastCode, lastErr
}

func postJSONOnce(ctx context.Context, url string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("webhook non-2xx: %s", resp.Status)
	}
	return resp.StatusCode, nil
}
