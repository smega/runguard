package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Email verification. A new account works right away (jobs, heartbeats,
// webhooks), but RunGuard sends no alert or summary emails to its address
// until the owner follows the verification link, so a signup with someone
// else's address can't turn into daily mail for them.

const verifyEmailTTL = 48 * time.Hour

// canEmail reports whether RunGuard may send alert/summary emails to u.
func canEmail(u *User) bool {
	return u != nil && !u.EmailUnverified
}

func (a *App) sendVerificationEmail(ctx context.Context, u *User) error {
	t, err := a.createToken(ctx, u.ID, "verify_email", verifyEmailTTL)
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}
	link := fmt.Sprintf("%s/verify-email?token=%s", a.cfg.App.BaseURL, t.Token)
	text := fmt.Sprintf(
		"Hi,\n\nPlease confirm this email address for your RunGuard account:\n\n%s\n\n"+
			"Until it's confirmed, RunGuard won't send alerts or summaries here "+
			"(webhook alerts work right away).\n\nThis link expires in 48 hours. "+
			"If you didn't sign up for RunGuard, ignore this email and you won't hear from us again.\n\n– RunGuard",
		link,
	)
	return a.sendMail(u.Email, "Confirm your email for RunGuard", text, "")
}

// markEmailVerified clears the unverified flag. Errors are only logged: the
// caller has already done its real job (logged the user in, reset a password).
func (a *App) markEmailVerified(ctx context.Context, userID primitive.ObjectID) {
	_, err := a.usersColl.UpdateByID(ctx, userID, bson.M{"$unset": bson.M{"email_unverified": ""}})
	if err != nil {
		log.Printf("mark email verified for %s: %v", userID.Hex(), err)
	}
}

// POST /api/auth/verify-email {"token": "..."}. Public: the link can be
// opened in a browser where the user isn't logged in.
func (a *App) handleVerifyEmail(c *gin.Context) {
	var req tokenConsumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var t Token
	err := a.tokensColl.FindOne(ctx, bson.M{"token": req.Token, "type": "verify_email"}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link"})
		return
	}
	if err != nil {
		log.Printf("verify email find token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	now := time.Now()
	if t.Used || now.After(t.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "link expired or already used"})
		return
	}

	if _, err := a.tokensColl.UpdateByID(ctx, t.ID, bson.M{"$set": bson.M{"used": true, "used_at": now}}); err != nil {
		log.Printf("verify email mark token used: %v", err)
	}
	a.markEmailVerified(ctx, t.UserID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/account/verify-email/resend (logged in).
func (a *App) handleResendVerification(c *gin.Context) {
	uVal, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	u := uVal.(*User)
	if canEmail(u) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "already_verified": true})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := a.sendVerificationEmail(ctx, u); err != nil {
		log.Printf("resend verification mail: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send email"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
