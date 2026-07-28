// summary.go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RunDailySummaryLoop runs in the background and sends daily emails.
func (a *App) RunDailySummaryLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := a.sendDueSummaries(); err != nil {
			log.Printf("daily summary: %v", err)
		}
	}
}

func (a *App) sendDueSummaries() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC()

	// Find users who might need a summary
	cur, err := a.usersColl.Find(ctx, bson.M{
		"summary_email_enabled": true,
	})
	if err != nil {
		return fmt.Errorf("find users for summary: %w", err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var u User
		if err := cur.Decode(&u); err != nil {
			log.Printf("daily summary decode user: %v", err)
			continue
		}

		plan := a.getUserPlanFeatures(&u)
		if !plan.AllowDailySummary {
			continue
		}

		hour := u.SummaryHourUTC
		if hour < 0 || hour > 23 {
			hour = 8
		}

		if now.Hour() != hour {
			continue
		}

		// Only once per UTC day
		if u.SummaryLastSentAt != nil {
			last := u.SummaryLastSentAt.UTC()
			if last.Year() == now.Year() && last.YearDay() == now.YearDay() {
				continue
			}
		}

		if err := a.sendUserDailySummary(ctx, &u, plan, now); err != nil {
			log.Printf("daily summary for %s: %v", u.Email, err)
			continue
		}

		// Mark as sent
		_, err := a.usersColl.UpdateByID(ctx, u.ID, bson.M{
			"$set": bson.M{"summary_last_sent_at": now},
		})
		if err != nil {
			log.Printf("daily summary update last_sent for %s: %v", u.Email, err)
		}
	}
	if err := cur.Err(); err != nil {
		return err
	}
	return nil
}

func (a *App) sendUserDailySummary(ctx context.Context, u *User, plan PlanFeatures, now time.Time) error {
	// 1) load user's jobs
	jobsCur, err := a.jobsColl.Find(ctx, bson.M{
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	})
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}
	defer jobsCur.Close(ctx)

	type jobState struct {
		Job   Job
		State string
	}

	var allJobs []jobState

	for jobsCur.Next(ctx) {
		var j Job
		if err := jobsCur.Decode(&j); err != nil {
			log.Printf("daily summary decode job: %v", err)
			continue
		}
		state := computeJobStatus(&j, now)
		allJobs = append(allJobs, jobState{Job: j, State: state})
	}
	if err := jobsCur.Err(); err != nil {
		return fmt.Errorf("jobs cursor: %w", err)
	}

	if len(allJobs) == 0 {
		return nil
	}

	// 2) late runs in the last 24 hours
	since := now.Add(-24 * time.Hour)
	runsCur, err := a.jobRunsColl.Find(ctx, bson.M{
		"user_id": u.ID,
		"status":  "late",
		"at": bson.M{
			"$gte": since,
			"$lte": now,
		},
	})
	if err != nil {
		return fmt.Errorf("late runs find: %w", err)
	}
	defer runsCur.Close(ctx)

	lateCounts := map[primitive.ObjectID]int{}

	for runsCur.Next(ctx) {
		var r JobRun
		if err := runsCur.Decode(&r); err != nil {
			log.Printf("daily summary decode run: %v", err)
			continue
		}
		lateCounts[r.JobID]++
	}
	if err := runsCur.Err(); err != nil {
		return fmt.Errorf("late runs cursor: %w", err)
	}

	// 3) classify jobs
	var lateNow []Job
	var lateRecently []Job
	var neverSeen []Job

	for _, js := range allJobs {
		switch js.State {
		case "late":
			lateNow = append(lateNow, js.Job)
		case "never_seen":
			neverSeen = append(neverSeen, js.Job)
		}

		if lateCounts[js.Job.ID] > 0 && js.State != "late" {
			lateRecently = append(lateRecently, js.Job)
		}
	}

	// If there's nothing interesting, skip
	if len(lateNow) == 0 && len(lateRecently) == 0 && len(neverSeen) == 0 {
		return nil
	}

	body := buildSummaryEmailBody(u, plan, lateNow, lateRecently, neverSeen, lateCounts, now)

	subject := "RunGuard – daily job summary"

	// Reuse your existing Mailgun helper
	if err := a.sendMail(u.Email, subject, body, ""); err != nil {
		return fmt.Errorf("send summary mail: %w", err)
	}

	return nil
}

func buildSummaryEmailBody(
	u *User,
	plan PlanFeatures,
	lateNow []Job,
	lateRecently []Job,
	neverSeen []Job,
	lateCounts map[primitive.ObjectID]int,
	now time.Time,
) string {
	var sb strings.Builder

	dateStr := now.UTC().Format("2006-01-02")

	fmt.Fprintf(&sb, "Hi,\n\n")
	fmt.Fprintf(&sb, "Here is your RunGuard daily summary for %s (UTC).\n\n", dateStr)

	if len(lateNow) > 0 {
		sb.WriteString("Jobs late right now:\n")
		for _, j := range lateNow {
			last := "never"
			if j.LastHeartbeatAt != nil {
				last = j.LastHeartbeatAt.UTC().Format(time.RFC3339)
			}
			fmt.Fprintf(&sb, "  • %s (last heartbeat: %s)\n", j.Name, last)
		}
		sb.WriteString("\n")
	}

	if len(lateRecently) > 0 {
		sb.WriteString("Jobs that went late in the last 24 hours:\n")
		for _, j := range lateRecently {
			count := lateCounts[j.ID]
			if count <= 0 {
				count = 1
			}
			fmt.Fprintf(&sb, "  • %s (%d late run(s))\n", j.Name, count)
		}
		sb.WriteString("\n")
	}

	if len(neverSeen) > 0 {
		sb.WriteString("Jobs that have never reported a heartbeat:\n")
		for _, j := range neverSeen {
			fmt.Fprintf(&sb, "  • %s\n", j.Name)
		}
		sb.WriteString("\n")
	}

	fmt.Fprintf(&sb, "You are on the %s plan (max %d jobs).\n", plan.Label, plan.MaxJobs)
	if plan.ID == PlanFree {
		sb.WriteString("Free plan includes webhook alerts and this daily summary.\n")
	} else if plan.ID == PlanPlus {
		sb.WriteString("Plus plan includes per-job email alerts and public status links.\n")
	} else if plan.ID == PlanScale {
		sb.WriteString("Scale plan will later include PDF/Excel exports and multi-project support.\n")
	}

	sb.WriteString("\nBest,\nRunGuard\n")

	return sb.String()

}
