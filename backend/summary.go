// summary.go
package main

import (
	"context"
	"fmt"
	"log"
	"sort"
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

		if err := a.sendUserDailySummary(ctx, &u, now); err != nil {
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

func (a *App) sendUserDailySummary(ctx context.Context, u *User, now time.Time) error {
	if !canEmail(u) {
		return nil // address not verified yet, see verify_email.go
	}
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

	body := buildSummaryEmailBody(u, lateNow, lateRecently, neverSeen, lateCounts, now)

	subject := "RunGuard – daily job summary"

	// Reuse your existing Mailgun helper
	if err := a.sendMail(u.Email, subject, body, ""); err != nil {
		return fmt.Errorf("send summary mail: %w", err)
	}

	return nil
}

func buildSummaryEmailBody(
	u *User,
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

	sb.WriteString("\nBest,\nRunGuard\n")

	return sb.String()

}

// ---------- Weekly summary ----------
//
// Same delivery machinery as the daily summary (a background loop, a
// per-user opt-in flag, a "last sent" guard so it only goes out once), but
// weekly and shaped around a digest rather than a same-day alert list:
// uptime per job, the noisiest jobs, alert counts by severity, and job
// churn (created/deleted) over the last 7 days. Sent once per week, on
// Mondays UTC, at the user's configured summary hour.

// RunWeeklySummaryLoop runs in the background and sends weekly digest emails.
func (a *App) RunWeeklySummaryLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := a.sendDueWeeklySummaries(); err != nil {
			log.Printf("weekly summary: %v", err)
		}
	}
}

func (a *App) sendDueWeeklySummaries() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC()

	// Only on Mondays UTC -- a fixed, simple choice for v1 rather than a
	// per-user configurable day.
	if now.Weekday() != time.Monday {
		return nil
	}

	cur, err := a.usersColl.Find(ctx, bson.M{
		"weekly_summary_enabled": true,
	})
	if err != nil {
		return fmt.Errorf("find users for weekly summary: %w", err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var u User
		if err := cur.Decode(&u); err != nil {
			log.Printf("weekly summary decode user: %v", err)
			continue
		}

		hour := u.SummaryHourUTC
		if hour < 0 || hour > 23 {
			hour = 8
		}
		if now.Hour() != hour {
			continue
		}

		// Only once per ISO week
		if u.WeeklySummaryLastSentAt != nil {
			last := u.WeeklySummaryLastSentAt.UTC()
			lastYear, lastWeek := last.ISOWeek()
			nowYear, nowWeek := now.ISOWeek()
			if lastYear == nowYear && lastWeek == nowWeek {
				continue
			}
		}

		if err := a.sendUserWeeklySummary(ctx, &u, now); err != nil {
			log.Printf("weekly summary for %s: %v", u.Email, err)
			continue
		}

		if _, err := a.usersColl.UpdateByID(ctx, u.ID, bson.M{
			"$set": bson.M{"weekly_summary_last_sent_at": now},
		}); err != nil {
			log.Printf("weekly summary update last_sent for %s: %v", u.Email, err)
		}
	}
	if err := cur.Err(); err != nil {
		return err
	}
	return nil
}

type weeklyJobStat struct {
	Job    Job
	Window uptimeWindow
}

func (a *App) sendUserWeeklySummary(ctx context.Context, u *User, now time.Time) error {
	if !canEmail(u) {
		return nil // address not verified yet, see verify_email.go
	}
	from := now.Add(-7 * 24 * time.Hour)

	jobsCur, err := a.jobsColl.Find(ctx, bson.M{
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	})
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}
	defer jobsCur.Close(ctx)

	var stats []weeklyJobStat
	severityByJob := map[primitive.ObjectID]string{}
	createdThisWeek := 0

	for jobsCur.Next(ctx) {
		var j Job
		if err := jobsCur.Decode(&j); err != nil {
			log.Printf("weekly summary decode job: %v", err)
			continue
		}
		severityByJob[j.ID] = normalizeSeverity(j.Severity)
		if j.CreatedAt.After(from) {
			createdThisWeek++
		}

		w, err := a.computeUptimeWindow(ctx, j.ID, u.ID, 7, from, now)
		if err != nil {
			log.Printf("weekly summary uptime for job %s: %v", j.ID.Hex(), err)
			continue
		}
		stats = append(stats, weeklyJobStat{Job: j, Window: w})
	}
	if err := jobsCur.Err(); err != nil {
		return fmt.Errorf("jobs cursor: %w", err)
	}

	if len(stats) == 0 {
		return nil
	}

	// Jobs soft-deleted in the last 7 days.
	deletedThisWeek, err := a.jobsColl.CountDocuments(ctx, bson.M{
		"user_id":    u.ID,
		"deleted_at": bson.M{"$gte": from, "$lte": now},
	})
	if err != nil {
		log.Printf("weekly summary count deleted jobs: %v", err)
		deletedThisWeek = 0
	}

	// Incidents opened this week, grouped by the owning job's severity.
	incCur, err := a.incidentsColl.Find(ctx, bson.M{
		"user_id":   u.ID,
		"opened_at": bson.M{"$gte": from, "$lte": now},
	})
	alertsBySeverity := map[string]int{}
	if err != nil {
		log.Printf("weekly summary find incidents: %v", err)
	} else {
		defer incCur.Close(ctx)
		for incCur.Next(ctx) {
			var inc Incident
			if err := incCur.Decode(&inc); err != nil {
				log.Printf("weekly summary decode incident: %v", err)
				continue
			}
			sev := severityByJob[inc.JobID]
			if sev == "" {
				sev = SeverityNormal
			}
			alertsBySeverity[sev]++
		}
	}

	// Noisiest jobs: sort by (late+fail) runs this week, descending.
	sortedStats := make([]weeklyJobStat, len(stats))
	copy(sortedStats, stats)
	sort.Slice(sortedStats, func(i, j int) bool {
		bi := sortedStats[i].Window.Late + sortedStats[i].Window.Fail
		bj := sortedStats[j].Window.Late + sortedStats[j].Window.Fail
		return bi > bj
	})

	body := buildWeeklySummaryEmailBody(u, sortedStats, alertsBySeverity, createdThisWeek, int(deletedThisWeek), now)
	subject := "RunGuard – weekly job summary"

	if err := a.sendMail(u.Email, subject, body, ""); err != nil {
		return fmt.Errorf("send weekly summary mail: %w", err)
	}
	return nil
}

func buildWeeklySummaryEmailBody(
	u *User,
	sortedStats []weeklyJobStat,
	alertsBySeverity map[string]int,
	createdThisWeek int,
	deletedThisWeek int,
	now time.Time,
) string {
	var sb strings.Builder

	weekStr := now.UTC().Format("2006-01-02")
	fmt.Fprintf(&sb, "Hi,\n\n")
	fmt.Fprintf(&sb, "Here is your RunGuard weekly summary for the week ending %s (UTC).\n\n", weekStr)

	fmt.Fprintf(&sb, "Uptime this week (%d job(s)):\n", len(sortedStats))
	for _, s := range sortedStats {
		fmt.Fprintf(&sb, "  • %s: %.1f%% (%d ok, %d late, %d fail)\n",
			s.Job.Name, s.Window.UptimePercent, s.Window.Ok, s.Window.Late, s.Window.Fail)
	}
	sb.WriteString("\n")

	noisiest := sortedStats
	if len(noisiest) > 5 {
		noisiest = noisiest[:5]
	}
	hasNoise := false
	for _, s := range noisiest {
		if s.Window.Late+s.Window.Fail > 0 {
			hasNoise = true
			break
		}
	}
	if hasNoise {
		sb.WriteString("Noisiest jobs this week:\n")
		for _, s := range noisiest {
			bad := s.Window.Late + s.Window.Fail
			if bad == 0 {
				continue
			}
			fmt.Fprintf(&sb, "  • %s (%d late, %d fail)\n", s.Job.Name, s.Window.Late, s.Window.Fail)
		}
		sb.WriteString("\n")
	}

	totalAlerts := 0
	for _, n := range alertsBySeverity {
		totalAlerts += n
	}
	fmt.Fprintf(&sb, "Alerts this week: %d total", totalAlerts)
	if totalAlerts > 0 {
		fmt.Fprintf(&sb, " (critical: %d, high: %d, normal: %d, low: %d)",
			alertsBySeverity[SeverityCritical], alertsBySeverity[SeverityHigh],
			alertsBySeverity[SeverityNormal], alertsBySeverity[SeverityLow])
	}
	sb.WriteString("\n\n")

	fmt.Fprintf(&sb, "Jobs created this week: %d\n", createdThisWeek)
	fmt.Fprintf(&sb, "Jobs deleted this week: %d\n", deletedThisWeek)

	sb.WriteString("\nBest,\nRunGuard\n")

	return sb.String()
}
