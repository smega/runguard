// admin.go
//
// In-app admin management, replacing the old "run a mongosh command by
// hand" approach:
//   - bootstrapAdmins promotes any user listed in config.yml's
//     admin.bootstrap_emails on every startup (idempotent, safe to leave
//     permanently configured).
//   - requireAdmin gates the admin API to users who are already admins.
//   - promote/demote are audited in the admin_audit_log collection so
//     there's a record of who granted/revoked admin access and when.
//   - demoting the last remaining admin is blocked, so the account can't
//     be locked out of its own admin tooling.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// bootstrapAdmins ensures every email in cfg.Admin.BootstrapEmails is
// marked is_admin=true. Called once at startup. It never removes admin
// from anyone — it's purely additive, so it can't be used to accidentally
// lock someone out via a config change.
func bootstrapAdmins(ctx context.Context, app *App) {
	for _, email := range app.cfg.Admin.BootstrapEmails {
		if email == "" {
			continue
		}
		res, err := app.usersColl.UpdateOne(ctx,
			bson.M{"email": email, "is_admin": bson.M{"$ne": true}},
			bson.M{"$set": bson.M{"is_admin": true}},
		)
		if err != nil {
			log.Printf("[admin bootstrap] update %s: %v", email, err)
			continue
		}
		if res.ModifiedCount > 0 {
			log.Printf("[admin bootstrap] granted admin to %s", email)
		}
	}
}

// requireAdmin must run after authMiddleware(); it assumes "user" is
// already set in the context.
func (a *App) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := c.MustGet("user").(*User)
		if !ok || u == nil || !u.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

func (a *App) writeAdminAudit(ctx context.Context, actor *User, action string, target *User) {
	entry := AdminAuditEntry{
		At:          time.Now().UTC(),
		ActorID:     actor.ID,
		ActorEmail:  actor.Email,
		Action:      action,
		TargetID:    target.ID,
		TargetEmail: target.Email,
	}
	if _, err := a.adminAuditColl.InsertOne(ctx, &entry); err != nil {
		log.Printf("[admin audit] insert failed (action=%s actor=%s target=%s): %v",
			action, actor.Email, target.Email, err)
	}
}

type adminUserDTO struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	CreatedAt     time.Time  `json:"created_at"`
	Plan          string     `json:"plan"`
	IsAdmin       bool       `json:"is_admin"`
	BillingStatus string     `json:"billing_status,omitempty"`
	TrialEndsAt   *time.Time `json:"trial_ends_at,omitempty"`
}

// handleAdminListUsers returns a simple list of all users for the admin
// panel. RunGuard is small enough today that this doesn't need real
// pagination yet; ?q= does a basic case-insensitive email filter.
func (a *App) handleAdminListUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if q := c.Query("q"); q != "" {
		filter["email"] = bson.M{"$regex": q, "$options": "i"}
	}

	cur, err := a.usersColl.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(500),
	)
	if err != nil {
		log.Printf("admin list users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer cur.Close(ctx)

	var users []User
	if err := cur.All(ctx, &users); err != nil {
		log.Printf("admin list users cursor: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	out := make([]adminUserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, adminUserDTO{
			ID:            u.ID.Hex(),
			Email:         u.Email,
			CreatedAt:     u.CreatedAt,
			Plan:          u.Plan,
			IsAdmin:       u.IsAdmin,
			BillingStatus: effectiveBillingStatus(&u),
			TrialEndsAt:   u.TrialEndsAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (a *App) countAdmins(ctx context.Context) (int64, error) {
	return a.usersColl.CountDocuments(ctx, bson.M{"is_admin": true})
}

func (a *App) handleAdminPromoteUser(c *gin.Context) {
	actor := c.MustGet("user").(*User)

	targetID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var target User
	if err := a.usersColl.FindOne(ctx, bson.M{"_id": targetID}).Decode(&target); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if target.IsAdmin {
		c.JSON(http.StatusOK, gin.H{"ok": true, "already_admin": true})
		return
	}

	if _, err := a.usersColl.UpdateByID(ctx, target.ID, bson.M{"$set": bson.M{"is_admin": true}}); err != nil {
		log.Printf("admin promote: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	a.writeAdminAudit(ctx, actor, "promote", &target)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleAdminDemoteUser(c *gin.Context) {
	actor := c.MustGet("user").(*User)

	targetID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if targetID == actor.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "you can't demote yourself — ask another admin to do it"})
		return
	}

	var target User
	if err := a.usersColl.FindOne(ctx, bson.M{"_id": targetID}).Decode(&target); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if !target.IsAdmin {
		c.JSON(http.StatusOK, gin.H{"ok": true, "already_not_admin": true})
		return
	}

	adminCount, err := a.countAdmins(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if adminCount <= 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can't remove the last remaining admin"})
		return
	}

	if _, err := a.usersColl.UpdateByID(ctx, target.ID, bson.M{"$set": bson.M{"is_admin": false}}); err != nil {
		log.Printf("admin demote: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	a.writeAdminAudit(ctx, actor, "demote", &target)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleAdminAuditLog(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := a.adminAuditColl.Find(ctx, bson.M{}, options.Find().
		SetSort(bson.D{{Key: "at", Value: -1}}).
		SetLimit(200),
	)
	if err != nil {
		log.Printf("admin audit log: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer cur.Close(ctx)

	var entries []AdminAuditEntry
	if err := cur.All(ctx, &entries); err != nil {
		log.Printf("admin audit log cursor: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}
