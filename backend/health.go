// health.go
//
// A monitoring product that can't tell you whether it's up itself is a bit
// of a credibility gap. This gives both an infra-facing health check
// (/healthz — for a load balancer, systemd, or an uptime check pointed at
// RunGuard itself) and a public JSON status endpoint the frontend's status
// page reads from.
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type healthStatus struct {
	OK        bool      `json:"ok"`
	Time      time.Time `json:"time"`
	Database  string    `json:"database"` // "ok" | "unreachable"
	VersionAt string    `json:"-"`
}

func (a *App) checkHealth(parent context.Context) healthStatus {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	status := healthStatus{Time: time.Now().UTC(), Database: "ok"}
	if err := a.db.Client().Ping(ctx, nil); err != nil {
		status.Database = "unreachable"
	}
	status.OK = status.Database == "ok"
	return status
}

// handleHealthz is meant for infrastructure (load balancer / systemd
// watchdog / an external uptime check), not end users: plain 200/503.
func (a *App) handleHealthz(c *gin.Context) {
	status := a.checkHealth(c.Request.Context())
	if !status.OK {
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	c.JSON(http.StatusOK, status)
}

// handlePublicStatus is the same check, served under /api/public so the
// frontend's status page can call it like any other API endpoint.
func (a *App) handlePublicStatus(c *gin.Context) {
	status := a.checkHealth(c.Request.Context())
	code := http.StatusOK
	if !status.OK {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, status)
}
