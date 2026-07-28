package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OverviewJobAttention struct {
	ID              primitive.ObjectID `json:"id"`
	Name            string             `json:"name"`
	Status          string             `json:"status"`
	LastHeartbeatAt *time.Time         `json:"last_heartbeat_at,omitempty"`
}

type OverviewStats struct {
	TotalJobs        int64                  `json:"totalJobs"`
	OkJobs           int64                  `json:"okJobs"`
	LateJobs         int64                  `json:"lateJobs"`
	NeverSeenJobs    int64                  `json:"neverSeenJobs"`
	IncidentCount24h int64                  `json:"incidentCount24h"`
	LastIncidentAt   *time.Time             `json:"lastIncidentAt,omitempty"`
	JobsNeedingAttn  []OverviewJobAttention `json:"jobsNeedingAttention"`
}

func (a *App) handleOverview(c *gin.Context) {
	u := c.MustGet("user").(*User)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC()

	// 🔑 Match handleListJobs: only non-deleted jobs for this user
	filter := bson.M{
		"user_id":    u.ID,
		"deleted_at": bson.M{"$exists": false},
	}

	cur, err := a.jobsColl.Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load jobs"})
		return
	}
	defer cur.Close(ctx)

	var (
		totalJobs     int64
		okJobs        int64
		lateJobs      int64
		neverSeenJobs int64
		jobIDs        []primitive.ObjectID
		attnJobs      []OverviewJobAttention
	)

	for cur.Next(ctx) {
		var j Job
		if err := cur.Decode(&j); err != nil {
			continue
		}

		totalJobs++
		jobIDs = append(jobIDs, j.ID)

		status := computeJobStatus(&j, now)

		switch status {
		case "ok":
			okJobs++
		case "late":
			lateJobs++
		case "never_seen":
			neverSeenJobs++
		}

		if status == "late" || status == "never_seen" {
			attnJobs = append(attnJobs, OverviewJobAttention{
				ID:              j.ID,
				Name:            j.Name,
				Status:          status,
				LastHeartbeatAt: j.LastHeartbeatAt,
			})
		}
	}

	if err := cur.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read jobs"})
		return
	}

	// No jobs → short-circuit
	if len(jobIDs) == 0 {
		c.JSON(http.StatusOK, &OverviewStats{
			TotalJobs:        0,
			OkJobs:           0,
			LateJobs:         0,
			NeverSeenJobs:    0,
			IncidentCount24h: 0,
			LastIncidentAt:   nil,
			JobsNeedingAttn:  []OverviewJobAttention{},
		})
		return
	}

	// Incidents in the last 24h (late runs)
	since := now.Add(-24 * time.Hour)

	incidentFilter := bson.M{
		"job_id":  bson.M{"$in": jobIDs},
		"user_id": u.ID,
		"status":  "late",
		"at":      bson.M{"$gte": since},
	}
	incidentCount24h, _ := a.jobRunsColl.CountDocuments(ctx, incidentFilter)

	// Last incident (latest late run)
	var lastIncidentAt *time.Time
	var lastRun struct {
		At time.Time `bson:"at"`
	}
	if err := a.jobRunsColl.FindOne(
		ctx,
		bson.M{
			"job_id":  bson.M{"$in": jobIDs},
			"user_id": u.ID,
			"status":  "late",
		},
		// you can add sort if you import options:
		// options.FindOne().SetSort(bson.D{{Key: "at", Value: -1}}),
	).Decode(&lastRun); err == nil {
		lastIncidentAt = &lastRun.At
	}

	c.JSON(http.StatusOK, &OverviewStats{
		TotalJobs:        totalJobs,
		OkJobs:           okJobs,
		LateJobs:         lateJobs,
		NeverSeenJobs:    neverSeenJobs,
		IncidentCount24h: incidentCount24h,
		LastIncidentAt:   lastIncidentAt,
		JobsNeedingAttn:  attnJobs,
	})
}
