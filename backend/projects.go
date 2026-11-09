package main

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongo "go.mongodb.org/mongo-driver/mongo"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

type projectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Environment string `json:"environment"`
}

type projectResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   string    `json:"description,omitempty"`
	Environment   string    `json:"environment,omitempty"`
	JobCount      int64     `json:"job_count"`
	OpenIncidents int64     `json:"open_incidents"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func projectSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "project"
	}
	return s
}

func (a *App) ensureUniqueProjectSlug(ctx context.Context, userID primitive.ObjectID, slug string, excludeID *primitive.ObjectID) string {
	base := slug
	for i := 1; i < 1000; i++ {
		filter := bson.M{"user_id": userID, "slug": slug, "deleted_at": bson.M{"$exists": false}}
		if excludeID != nil {
			filter["_id"] = bson.M{"$ne": *excludeID}
		}
		count, _ := a.projectsColl.CountDocuments(ctx, filter)
		if count == 0 {
			return slug
		}
		slug = base + "-" + strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(string(rune('0'+(i%10)))), "0"))
	}
	return base + "-x"
}

func (a *App) handleListProjects(c *gin.Context) {
	u := c.MustGet("user").(*User)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := a.projectsColl.Find(ctx, bson.M{"user_id": u.ID, "deleted_at": bson.M{"$exists": false}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load projects"})
		return
	}
	defer cur.Close(ctx)

	out := []projectResponse{}
	for cur.Next(ctx) {
		var p Project
		if err := cur.Decode(&p); err != nil {
			continue
		}
		jobCount, _ := a.jobsColl.CountDocuments(ctx, bson.M{"user_id": u.ID, "project_id": p.ID, "deleted_at": bson.M{"$exists": false}})
		openIncidents, _ := a.incidentsColl.CountDocuments(ctx, bson.M{"user_id": u.ID, "project_id": p.ID, "state": bson.M{"$in": []string{IncidentStateOpen, IncidentStateAcknowledged}}})
		out = append(out, projectResponse{ID: p.ID.Hex(), Name: p.Name, Slug: p.Slug, Description: p.Description, Environment: p.Environment, JobCount: jobCount, OpenIncidents: openIncidents, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"projects": out})
}

func (a *App) handleCreateProject(c *gin.Context) {
	u := c.MustGet("user").(*User)
	var req projectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if !u.IsAdmin && a.cfg.Limits.MaxProjectsPerUser > 0 {
		count, _ := a.projectsColl.CountDocuments(ctx, bson.M{"user_id": u.ID, "deleted_at": bson.M{"$exists": false}})
		if int(count) >= a.cfg.Limits.MaxProjectsPerUser {
			c.JSON(http.StatusForbidden, gin.H{"error": "project limit reached", "max_projects": a.cfg.Limits.MaxProjectsPerUser})
			return
		}
	}
	now := time.Now().UTC()
	slug := projectSlug(req.Name)
	// simplified uniqueness with timestamp suffix if needed
	if n, _ := a.projectsColl.CountDocuments(ctx, bson.M{"user_id": u.ID, "slug": slug, "deleted_at": bson.M{"$exists": false}}); n > 0 {
		slug = slug + "-" + now.Format("150405")
	}
	proj := Project{UserID: u.ID, Name: strings.TrimSpace(req.Name), Slug: slug, Description: strings.TrimSpace(req.Description), Environment: strings.TrimSpace(req.Environment), CreatedAt: now, UpdatedAt: now}
	res, err := a.projectsColl.InsertOne(ctx, proj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create project"})
		return
	}
	proj.ID = res.InsertedID.(primitive.ObjectID)
	c.JSON(http.StatusCreated, gin.H{"project": projectResponse{ID: proj.ID.Hex(), Name: proj.Name, Slug: proj.Slug, Description: proj.Description, Environment: proj.Environment, JobCount: 0, OpenIncidents: 0, CreatedAt: proj.CreatedAt, UpdatedAt: proj.UpdatedAt}})
}

func (a *App) handleUpdateProject(c *gin.Context) {
	u := c.MustGet("user").(*User)
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	var req projectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	var existing Project
	if err := a.projectsColl.FindOne(ctx, bson.M{"_id": id, "user_id": u.ID, "deleted_at": bson.M{"$exists": false}}).Decode(&existing); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load project"})
		return
	}
	update := bson.M{"updated_at": time.Now().UTC()}
	if strings.TrimSpace(req.Name) != "" {
		update["name"] = strings.TrimSpace(req.Name)
		update["slug"] = projectSlug(req.Name)
	}
	update["description"] = strings.TrimSpace(req.Description)
	update["environment"] = strings.TrimSpace(req.Environment)
	if _, err := a.projectsColl.UpdateByID(ctx, id, bson.M{"$set": update}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update project"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleDeleteProject(c *gin.Context) {
	u := c.MustGet("user").(*User)
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	jobsCount, _ := a.jobsColl.CountDocuments(ctx, bson.M{"user_id": u.ID, "project_id": id, "deleted_at": bson.M{"$exists": false}})
	if jobsCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project still has jobs assigned"})
		return
	}
	now := time.Now().UTC()
	res, err := a.projectsColl.UpdateOne(ctx, bson.M{"_id": id, "user_id": u.ID, "deleted_at": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"deleted_at": now, "updated_at": now}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete project"})
		return
	}
	if res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
