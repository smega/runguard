package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

type updateAccountRequest struct {
	SummaryEmailEnabled  *bool `json:"summary_email_enabled,omitempty"`
	SummaryHourUTC       *int  `json:"summary_hour_utc,omitempty"`
	WeeklySummaryEnabled *bool `json:"weekly_summary_enabled,omitempty"`
}
type accountResponse struct {
	Email                   string     `json:"email"`
	SummaryEmailEnabled     bool       `json:"summary_email_enabled"`
	SummaryHourUTC          int        `json:"summary_hour_utc"`
	SummaryLastSentAt       *time.Time `json:"summary_last_sent_at,omitempty"`
	WeeklySummaryEnabled    bool       `json:"weekly_summary_enabled"`
	WeeklySummaryLastSentAt *time.Time `json:"weekly_summary_last_sent_at,omitempty"`
	// MaxJobs/MaxProjects are 0 when unlimited (the self-hosted default) —
	// RunGuard has no plan/tier system, just this one optional cap.
	MaxJobs     int  `json:"max_jobs"`
	MaxProjects int  `json:"max_projects"`
	IsAdmin     bool `json:"is_admin"`
	// EmailUnverified: no alert/summary emails until the address is
	// confirmed (see verify_email.go).
	EmailUnverified bool `json:"email_unverified"`
}

func accountResponseFromUser(a *App, u *User) accountResponse {
	// Admins have no job/project limit (see handleCreateJob, handleAgentRegister,
	// handleCreateProject). Report 0 ("unlimited") for them too, otherwise the
	// frontend enforces the regular cap on its own and blocks admins anyway.
	maxJobs, maxProjects := a.cfg.Limits.MaxJobsPerUser, a.cfg.Limits.MaxProjectsPerUser
	if u.IsAdmin {
		maxJobs, maxProjects = 0, 0
	}
	return accountResponse{
		Email:                   u.Email,
		SummaryEmailEnabled:     u.SummaryEmailEnabled,
		SummaryHourUTC:          u.SummaryHourUTC,
		SummaryLastSentAt:       u.SummaryLastSentAt,
		WeeklySummaryEnabled:    u.WeeklySummaryEnabled,
		WeeklySummaryLastSentAt: u.WeeklySummaryLastSentAt,
		MaxJobs:                 maxJobs,
		MaxProjects:             maxProjects,
		IsAdmin:                 u.IsAdmin,
		EmailUnverified:         u.EmailUnverified,
	}
}

func (a *App) handleGetAccount(c *gin.Context) {
	u := c.MustGet("user").(*User)
	c.JSON(http.StatusOK, accountResponseFromUser(a, u))
}
func (a *App) handleUpdateAccount(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req updateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	update := bson.M{}
	if req.SummaryEmailEnabled != nil {
		update["summary_email_enabled"] = *req.SummaryEmailEnabled
	}
	if req.SummaryHourUTC != nil {
		h := *req.SummaryHourUTC
		if h < 0 {
			h = 0
		}
		if h > 23 {
			h = 23
		}
		update["summary_hour_utc"] = h
	}
	if req.WeeklySummaryEnabled != nil {
		update["weekly_summary_enabled"] = *req.WeeklySummaryEnabled
	}
	if len(update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if _, err := a.usersColl.UpdateByID(ctx, u.ID, bson.M{"$set": update}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	var fresh User
	if err := a.usersColl.FindOne(ctx, bson.M{"_id": u.ID}).Decode(&fresh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload user"})
		return
	}
	c.JSON(http.StatusOK, accountResponseFromUser(a, &fresh))
}
