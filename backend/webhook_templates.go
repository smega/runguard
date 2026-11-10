// webhook_templates.go
package main

import (
	"context"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type createWebhookTemplateRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type"` // "generic" | "slack" | "discord" | "pagerduty" | "teams" | "custom" (optional)
	// Body is a Go text/template source, required (and only used) when
	// Type == "custom" -- see buildWebhookBodyForType in alerts.go.
	Body string `json:"body,omitempty"`
}

func (a *App) handleListWebhookTemplates(c *gin.Context) {
	uVal, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user := uVal.(*User)

	ctx := c.Request.Context()
	filter := bson.M{"user_id": user.ID}

	// ✅ use options.Find() instead of mongo.FindOptions
	findOpts := options.Find().SetSort(bson.D{
		{Key: "created_at", Value: 1},
	})

	cur, err := a.webhookTemplatesColl.Find(ctx, filter, findOpts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer cur.Close(ctx)

	var templates []WebhookTemplate
	if err := cur.All(ctx, &templates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (a *App) handleCreateWebhookTemplate(c *gin.Context) {
	uVal, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user := uVal.(*User)

	var req createWebhookTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	name := strings.TrimSpace(req.Name)
	url := strings.TrimSpace(req.URL)
	if name == "" || url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and url are required"})
		return
	}
	if err := validateWebhookURLScheme(url); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tType := normalizeWebhookTemplateType(req.Type)

	body := ""
	if tType == WebhookTemplateTypeCustom {
		body = req.Body
		if strings.TrimSpace(body) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body is required for a custom template"})
			return
		}
		if _, err := template.New("webhook").Parse(body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template: " + err.Error()})
			return
		}
	}

	now := time.Now().UTC()
	tmpl := WebhookTemplate{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Name:      name,
		URL:       url,
		Type:      tType,
		Body:      body,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := a.webhookTemplatesColl.InsertOne(c.Request.Context(), tmpl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"template": tmpl})
}

func (a *App) handleDeleteWebhookTemplate(c *gin.Context) {
	uVal, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user := uVal.(*User)

	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()

	var tmpl WebhookTemplate
	err = a.webhookTemplatesColl.
		FindOneAndDelete(ctx, bson.M{"_id": id, "user_id": user.ID}).
		Decode(&tmpl)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// best-effort: remove this URL from all jobs for this user
	if tmpl.URL != "" {
		_, _ = a.jobsColl.UpdateMany(
			ctx,
			bson.M{"user_id": user.ID},
			bson.M{"$pull": bson.M{"alert_webhook_urls": tmpl.URL}},
		)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

const (
	WebhookTemplateTypeGeneric   = "generic"
	WebhookTemplateTypeSlack     = "slack"
	WebhookTemplateTypeDiscord   = "discord"
	WebhookTemplateTypePagerDuty = "pagerduty"
	WebhookTemplateTypeTeams     = "teams"
	WebhookTemplateTypeCustom    = "custom"
)

func normalizeWebhookTemplateType(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch s {
	case WebhookTemplateTypeSlack:
		return WebhookTemplateTypeSlack
	case WebhookTemplateTypeDiscord:
		return WebhookTemplateTypeDiscord
	case WebhookTemplateTypePagerDuty:
		return WebhookTemplateTypePagerDuty
	case WebhookTemplateTypeTeams:
		return WebhookTemplateTypeTeams
	case WebhookTemplateTypeCustom:
		return WebhookTemplateTypeCustom
	case "", WebhookTemplateTypeGeneric:
		fallthrough
	default:
		return WebhookTemplateTypeGeneric
	}
}

func (a *App) findWebhookTemplateByURL(ctx context.Context, userID primitive.ObjectID, url string) (*WebhookTemplate, error) {
	var tmpl WebhookTemplate
	err := a.webhookTemplatesColl.FindOne(ctx, bson.M{
		"user_id": userID,
		"url":     url,
	}).Decode(&tmpl)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &tmpl, nil
}
