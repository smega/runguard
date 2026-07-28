// exports.go
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf" // or the maintained fork if you prefer
	"github.com/xuri/excelize/v2"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// maxExportRows caps how many jobs a single export request will render.
// Plan limits already keep normal accounts well under this, but an export
// shouldn't be able to become an unbounded, on-demand load generator (e.g.
// for an admin account, which isn't capped by a plan's max_jobs).
const maxExportRows = 5000

// one row in the export
type jobExportRow struct {
	Name                 string
	Status               string
	LastHeartbeatAt      string
	ExpectedIntervalSecs int
	GraceSecs            int
	AlertEmailEnabled    bool
	AlertWebhookEnabled  bool
	PublicEnabled        bool
	CreatedAt            string
	UpdatedAt            string
}

// internal helper: load current user's jobs and flatten into rows
func (a *App) buildJobExportRows(ctx context.Context, u *User, now time.Time) ([]jobExportRow, error) {
	filter := bson.M{
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}

	cur, err := a.jobsColl.Find(ctx, filter, options.Find().SetLimit(maxExportRows))
	if err != nil {
		return nil, fmt.Errorf("find jobs for export: %w", err)
	}
	defer cur.Close(ctx)

	var rows []jobExportRow

	for cur.Next(ctx) {
		var j Job
		if err := cur.Decode(&j); err != nil {
			log.Printf("export decode job: %v", err)
			continue
		}

		status := computeJobStatus(&j, now)

		var lastHB, createdAt, updatedAt string
		if j.LastHeartbeatAt != nil && !j.LastHeartbeatAt.IsZero() {
			lastHB = j.LastHeartbeatAt.UTC().Format(time.RFC3339)
		}
		if !j.CreatedAt.IsZero() {
			createdAt = j.CreatedAt.UTC().Format(time.RFC3339)
		}
		if !j.UpdatedAt.IsZero() {
			updatedAt = j.UpdatedAt.UTC().Format(time.RFC3339)
		}

		rows = append(rows, jobExportRow{
			Name:                 j.Name,
			Status:               status,
			LastHeartbeatAt:      lastHB,
			ExpectedIntervalSecs: j.ExpectedIntervalSecs,
			GraceSecs:            j.GraceSecs,
			AlertEmailEnabled:    j.AlertEmailEnabled,
			AlertWebhookEnabled:  j.AlertWebhookEnabled,
			PublicEnabled:        j.PublicEnabled,
			CreatedAt:            createdAt,
			UpdatedAt:            updatedAt,
		})
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("export jobs cursor: %w", err)
	}

	return rows, nil
}

// ---------- CSV ----------

// GET /api/exports/jobs.csv
func (a *App) handleExportJobsCSV(c *gin.Context) {
	u := c.MustGet("user").(*User)
	plan := a.getUserPlanFeatures(u)

	// for safety you *could* re-check admin explicitly, but the helper already did it
	if !plan.AllowExports {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "exports are only available on the top tier",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC()
	rows, err := a.buildJobExportRows(ctx, u, now)
	if err != nil {
		log.Printf("export csv build rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build export"})
		return
	}

	filename := fmt.Sprintf("runguard_jobs_%s.csv", now.UTC().Format("20060102"))

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	w := csv.NewWriter(c.Writer)

	_ = w.Write([]string{
		"Name",
		"Status",
		"LastHeartbeatAt",
		"ExpectedIntervalSeconds",
		"GraceSeconds",
		"AlertEmailEnabled",
		"AlertWebhookEnabled",
		"PublicEnabled",
		"CreatedAt",
		"UpdatedAt",
	})

	for _, r := range rows {
		record := []string{
			r.Name,
			r.Status,
			r.LastHeartbeatAt,
			fmt.Sprintf("%d", r.ExpectedIntervalSecs),
			fmt.Sprintf("%d", r.GraceSecs),
			fmt.Sprintf("%t", r.AlertEmailEnabled),
			fmt.Sprintf("%t", r.AlertWebhookEnabled),
			fmt.Sprintf("%t", r.PublicEnabled),
			r.CreatedAt,
			r.UpdatedAt,
		}
		if err := w.Write(record); err != nil {
			log.Printf("export csv write record: %v", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		log.Printf("export csv flush: %v", err)
	}
}

// ---------- Excel (XLSX) ----------

// GET /api/exports/jobs.xlsx
func (a *App) handleExportJobsExcel(c *gin.Context) {
	u := c.MustGet("user").(*User)
	plan := a.getUserPlanFeatures(u)

	// for safety you *could* re-check admin explicitly, but the helper already did it
	if !plan.AllowExports {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "exports are only available on the top tier",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC()
	rows, err := a.buildJobExportRows(ctx, u, now)
	if err != nil {
		log.Printf("export xlsx build rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build export"})
		return
	}

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	headers := []string{
		"Name",
		"Status",
		"LastHeartbeatAt",
		"ExpectedIntervalSeconds",
		"GraceSeconds",
		"AlertEmailEnabled",
		"AlertWebhookEnabled",
		"PublicEnabled",
		"CreatedAt",
		"UpdatedAt",
	}

	// header row
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	// data rows
	for rowIdx, r := range rows {
		excelRow := rowIdx + 2 // 1-based, + header
		values := []interface{}{
			r.Name,
			r.Status,
			r.LastHeartbeatAt,
			r.ExpectedIntervalSecs,
			r.GraceSecs,
			r.AlertEmailEnabled,
			r.AlertWebhookEnabled,
			r.PublicEnabled,
			r.CreatedAt,
			r.UpdatedAt,
		}

		for colIdx, v := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		log.Printf("export xlsx write buffer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render excel"})
		return
	}

	filename := fmt.Sprintf("runguard_jobs_%s.xlsx", now.UTC().Format("20060102"))

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// ---------- PDF ----------

// GET /api/exports/jobs.pdf
func (a *App) handleExportJobsPDF(c *gin.Context) {
	u := c.MustGet("user").(*User)
	plan := a.getUserPlanFeatures(u)

	// for safety you *could* re-check admin explicitly, but the helper already did it
	if !plan.AllowExports {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "exports are only available on the top tier",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC()
	rows, err := a.buildJobExportRows(ctx, u, now)
	if err != nil {
		log.Printf("export pdf build rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build export"})
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("RunGuard jobs export", false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 11)

	// Title
	pdf.Cell(0, 8, fmt.Sprintf("RunGuard jobs – %s", now.UTC().Format("2006-01-02 15:04:05 MST")))
	pdf.Ln(10)

	// Simple table: Name | Status | Last heartbeat
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(70, 6, "Name", "1", 0, "", false, 0, "")
	pdf.CellFormat(25, 6, "Status", "1", 0, "", false, 0, "")
	pdf.CellFormat(0, 6, "Last heartbeat (UTC)", "1", 1, "", false, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	for _, r := range rows {
		// Name (wrapped)
		pdf.CellFormat(70, 6, r.Name, "1", 0, "", false, 0, "")
		pdf.CellFormat(25, 6, r.Status, "1", 0, "", false, 0, "")
		pdf.CellFormat(0, 6, r.LastHeartbeatAt, "1", 1, "", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		log.Printf("export pdf output: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render pdf"})
		return
	}

	filename := fmt.Sprintf("runguard_jobs_%s.pdf", now.UTC().Format("20060102"))

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

// ---------- Per-job run history export (CSV / XLSX) ----------
//
// Same idea as the account-wide jobs export above, but scoped to a single
// job's run history instead of the job list -- "export this job's runs" for
// someone debugging a specific job, rather than a snapshot of all jobs.
// Reuses the same maxExportRows cap and AllowExports plan gate.

type jobRunExportRow struct {
	At           string
	Status       string
	ExitCode     string
	DurationMs   int64
	Env          string
	Tags         string
	AgentVersion string
	LateBySec    int64
}

func (a *App) loadJobForExport(ctx context.Context, jobID string, u *User) (*Job, error) {
	id, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		return nil, fmt.Errorf("invalid job id")
	}
	var job Job
	err = a.jobsColl.FindOne(ctx, bson.M{
		"_id":        id,
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}).Decode(&job)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("job not found")
		}
		return nil, err
	}
	return &job, nil
}

func (a *App) buildJobRunExportRows(ctx context.Context, jobID primitive.ObjectID) ([]jobRunExportRow, error) {
	cur, err := a.jobRunsColl.Find(ctx, bson.M{"job_id": jobID},
		options.Find().SetSort(bson.D{{Key: "at", Value: -1}}).SetLimit(maxExportRows))
	if err != nil {
		return nil, fmt.Errorf("find job runs for export: %w", err)
	}
	defer cur.Close(ctx)

	var rows []jobRunExportRow
	for cur.Next(ctx) {
		var r JobRun
		if err := cur.Decode(&r); err != nil {
			log.Printf("export job runs decode: %v", err)
			continue
		}
		exitCode := ""
		if r.ExitCode != nil {
			exitCode = fmt.Sprintf("%d", *r.ExitCode)
		}
		rows = append(rows, jobRunExportRow{
			At:           r.At.UTC().Format(time.RFC3339),
			Status:       r.Status,
			ExitCode:     exitCode,
			DurationMs:   r.DurationMs,
			Env:          r.Env,
			Tags:         strings.Join(r.Tags, ";"),
			AgentVersion: r.AgentVersion,
			LateBySec:    r.LateBySec,
		})
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("export job runs cursor: %w", err)
	}
	return rows, nil
}

// GET /api/jobs/:id/export/runs.csv
func (a *App) handleExportJobRunsCSV(c *gin.Context) {
	u := c.MustGet("user").(*User)
	plan := a.getUserPlanFeatures(u)
	if !plan.AllowExports {
		c.JSON(http.StatusForbidden, gin.H{"error": "exports are only available on the top tier"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	job, err := a.loadJobForExport(ctx, c.Param("id"), u)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	rows, err := a.buildJobRunExportRows(ctx, job.ID)
	if err != nil {
		log.Printf("export job runs csv: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build export"})
		return
	}

	filename := fmt.Sprintf("runguard_%s_runs_%s.csv", safeFilenamePart(job.Name), time.Now().UTC().Format("20060102"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"At", "Status", "ExitCode", "DurationMs", "Env", "Tags", "AgentVersion", "LateBySeconds"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.At, r.Status, r.ExitCode,
			fmt.Sprintf("%d", r.DurationMs),
			r.Env, r.Tags, r.AgentVersion,
			fmt.Sprintf("%d", r.LateBySec),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		log.Printf("export job runs csv flush: %v", err)
	}
}

// GET /api/jobs/:id/export/runs.xlsx
func (a *App) handleExportJobRunsExcel(c *gin.Context) {
	u := c.MustGet("user").(*User)
	plan := a.getUserPlanFeatures(u)
	if !plan.AllowExports {
		c.JSON(http.StatusForbidden, gin.H{"error": "exports are only available on the top tier"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	job, err := a.loadJobForExport(ctx, c.Param("id"), u)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	rows, err := a.buildJobRunExportRows(ctx, job.ID)
	if err != nil {
		log.Printf("export job runs xlsx: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build export"})
		return
	}

	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	headers := []string{"At", "Status", "ExitCode", "DurationMs", "Env", "Tags", "AgentVersion", "LateBySeconds"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	for rowIdx, r := range rows {
		excelRow := rowIdx + 2
		values := []interface{}{r.At, r.Status, r.ExitCode, r.DurationMs, r.Env, r.Tags, r.AgentVersion, r.LateBySec}
		for colIdx, v := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		log.Printf("export job runs xlsx write buffer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render excel"})
		return
	}

	filename := fmt.Sprintf("runguard_%s_runs_%s.xlsx", safeFilenamePart(job.Name), time.Now().UTC().Format("20060102"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// safeFilenamePart turns a job name into something safe to drop into a
// Content-Disposition filename -- alphanumeric plus a few separators, capped
// in length, never empty.
func safeFilenamePart(name string) string {
	var sb strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r == '-' || r == '_':
			sb.WriteRune(r)
		case r == ' ':
			sb.WriteRune('_')
		}
	}
	out := sb.String()
	if len(out) > 40 {
		out = out[:40]
	}
	if out == "" {
		out = "job"
	}
	return out
}
