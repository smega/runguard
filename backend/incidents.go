package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type incidentResponse struct {
	ID              string     `json:"id"`
	JobID           string     `json:"job_id"`
	ProjectID       string     `json:"project_id,omitempty"`
	JobName         string     `json:"job_name,omitempty"`
	ProjectName     string     `json:"project_name,omitempty"`
	Kind            string     `json:"kind"`
	State           string     `json:"state"`
	Title           string     `json:"title"`
	Summary         string     `json:"summary,omitempty"`
	OpenedAt        time.Time  `json:"opened_at"`
	LastSeenAt      time.Time  `json:"last_seen_at"`
	AcknowledgedAt  *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	OccurrenceCount int        `json:"occurrence_count"`
	LastRunStatus   string     `json:"last_run_status,omitempty"`
}

type updateIncidentRequest struct {
	State string `json:"state"`
}

func (a *App) openOrRefreshIncident(ctx context.Context, job *Job, kind, summary, runStatus string, now time.Time) error {
	filter := bson.M{"user_id": job.UserID, "job_id": job.ID, "kind": kind, "state": bson.M{"$in": []string{IncidentStateOpen, IncidentStateAcknowledged}}}
	update := bson.M{
		"$set":         bson.M{"last_seen_at": now, "updated_at": now, "summary": summary, "last_run_status": runStatus},
		"$inc":         bson.M{"occurrence_count": 1},
		"$setOnInsert": bson.M{"opened_at": now, "created_at": now, "state": IncidentStateOpen, "title": fmt.Sprintf("%s incident: %s", strings.Title(kind), job.Name), "project_id": job.ProjectID, "user_id": job.UserID, "job_id": job.ID, "kind": kind},
	}
	_, err := a.incidentsColl.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (a *App) resolveOpenIncidents(ctx context.Context, job *Job, now time.Time) error {
	_, err := a.incidentsColl.UpdateMany(ctx, bson.M{"user_id": job.UserID, "job_id": job.ID, "state": bson.M{"$in": []string{IncidentStateOpen, IncidentStateAcknowledged}}}, bson.M{"$set": bson.M{"state": IncidentStateResolved, "resolved_at": now, "updated_at": now, "last_seen_at": now, "last_run_status": "ok"}})
	return err
}

func (a *App) incidentToResponse(ctx context.Context, in Incident) incidentResponse {
	resp := incidentResponse{ID: in.ID.Hex(), JobID: in.JobID.Hex(), Kind: in.Kind, State: in.State, Title: in.Title, Summary: in.Summary, OpenedAt: in.OpenedAt, LastSeenAt: in.LastSeenAt, AcknowledgedAt: in.AcknowledgedAt, ResolvedAt: in.ResolvedAt, OccurrenceCount: in.OccurrenceCount, LastRunStatus: in.LastRunStatus}
	if !in.ProjectID.IsZero() {
		resp.ProjectID = in.ProjectID.Hex()
	}
	var job Job
	if err := a.jobsColl.FindOne(ctx, bson.M{"_id": in.JobID}).Decode(&job); err == nil {
		resp.JobName = job.Name
	}
	if !in.ProjectID.IsZero() {
		var project Project
		if err := a.projectsColl.FindOne(ctx, bson.M{"_id": in.ProjectID}).Decode(&project); err == nil {
			resp.ProjectName = project.Name
		}
	}
	return resp
}

func (a *App) handleListIncidents(c *gin.Context) {
	u := c.MustGet("user").(*User)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	filter := bson.M{"user_id": u.ID}
	state := strings.TrimSpace(c.Query("state"))
	if state != "" {
		filter["state"] = state
	}
	cur, err := a.incidentsColl.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetLimit(100))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load incidents"})
		return
	}
	defer cur.Close(ctx)
	out := []incidentResponse{}
	for cur.Next(ctx) {
		var inc Incident
		if err := cur.Decode(&inc); err == nil {
			out = append(out, a.incidentToResponse(ctx, inc))
		}
	}
	c.JSON(http.StatusOK, gin.H{"incidents": out})
}

func (a *App) handleUpdateIncident(c *gin.Context) {
	u := c.MustGet("user").(*User)
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid incident id"})
		return
	}
	var req updateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	state := strings.ToLower(strings.TrimSpace(req.State))
	if state != IncidentStateAcknowledged && state != IncidentStateResolved && state != IncidentStateOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	set := bson.M{"state": state, "updated_at": time.Now().UTC()}
	now := time.Now().UTC()
	if state == IncidentStateAcknowledged {
		set["acknowledged_at"] = now
	}
	if state == IncidentStateResolved {
		set["resolved_at"] = now
	}
	res, err := a.incidentsColl.UpdateOne(ctx, bson.M{"_id": id, "user_id": u.ID}, bson.M{"$set": set})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update incident"})
		return
	}
	if res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
		return
	}
	var inc Incident
	if err := a.incidentsColl.FindOne(ctx, bson.M{"_id": id}).Decode(&inc); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload incident"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"incident": a.incidentToResponse(ctx, inc)})
}
