// auth.go
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type signupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,max=128"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type magicRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetConfirmRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,max=128"`
}

type tokenConsumeRequest struct {
	Token string `json:"token" binding:"required"`
}

type authClaims struct {
	UserID string `json:"uid"`
	// TV (token version) is compared against the user's current
	// TokenVersion in authMiddleware. If they don't match, the token has
	// been revoked (e.g. by a password change or "log out everywhere").
	TV int `json:"tv"`
	jwt.RegisteredClaims
}

// ---------- Signup / Login / Logout / Me ----------

const passwordRequirementsText = "Password must be 8-128 characters and include at least 1 letter, 1 number, and 1 special character."

func validatePasswordComplexity(password string) error {
	if l := utf8.RuneCountInString(password); l < 8 || l > 128 {
		return fmt.Errorf(passwordRequirementsText)
	}

	hasLetter := false
	hasNumber := false
	hasSpecial := false
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasLetter || !hasNumber || !hasSpecial {
		return fmt.Errorf(passwordRequirementsText)
	}
	return nil
}

func authPayloadError(err error) string {
	return "invalid payload"
}

func (a *App) handleSignup(c *gin.Context) {
	var req signupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": authPayloadError(err)})
		return
	}
	if err := validatePasswordComplexity(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var existing User
	err := a.usersColl.FindOne(ctx, bson.M{"email": req.Email}).Decode(&existing)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("find user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash pwd: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	now := time.Now()
	user := User{
		Email:               req.Email,
		PasswordHash:        string(hash),
		CreatedAt:           now,
		SummaryEmailEnabled: true,
		SummaryHourUTC:      8,
		EmailUnverified:     true,
	}
	res, err := a.usersColl.InsertOne(ctx, user)
	if err != nil {
		log.Printf("insert user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	user.ID = res.InsertedID.(primitive.ObjectID)

	if a.projectsColl != nil {
		_, err := a.projectsColl.InsertOne(ctx, Project{
			UserID:      user.ID,
			Name:        "General",
			Slug:        "general",
			Environment: "production",
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		if err != nil {
			log.Printf("insert default project: %v", err)
		}
	}

	// A failed send isn't fatal: the account works, and the user can resend
	// from the app.
	if err := a.sendVerificationEmail(ctx, &user); err != nil {
		log.Printf("send verification mail: %v", err)
	}

	if err := a.setAuthCookie(c, user.ID.Hex(), user.TokenVersion); err != nil {
		log.Printf("set cookie: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": gin.H{
			"id":    user.ID.Hex(),
			"email": user.Email,
		},
	})
}

func (a *App) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var user User
	err := a.usersColl.FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err != nil {
		log.Printf("find user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := a.setAuthCookie(c, user.ID.Hex(), user.TokenVersion); err != nil {
		log.Printf("set cookie: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":    user.ID.Hex(),
			"email": user.Email,
		},
	})
}

func (a *App) handleLogout(c *gin.Context) {
	cfg := &a.cfg.Auth
	c.SetCookie(cfg.CookieName, "", -1, "/", cfg.CookieDomain, cfg.CookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleLogoutAllDevices invalidates every previously issued token for this
// user (by bumping token_version) and then issues a fresh cookie for the
// current browser, so the caller stays logged in here while every other
// session/token is immediately revoked.
func (a *App) handleLogoutAllDevices(c *gin.Context) {
	u, ok := c.MustGet("user").(*User)
	if !ok || u == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user missing in context"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	res := a.usersColl.FindOneAndUpdate(
		ctx,
		bson.M{"_id": u.ID},
		bson.M{"$inc": bson.M{"token_version": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var updated User
	if err := res.Decode(&updated); err != nil {
		log.Printf("logout-all update user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if err := a.setAuthCookie(c, updated.ID.Hex(), updated.TokenVersion); err != nil {
		log.Printf("logout-all set cookie: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleMe(c *gin.Context) {
	u, ok := c.MustGet("user").(*User)
	if !ok || u == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user missing in context"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":                    u.ID.Hex(),
			"email":                 u.Email,
			"created_at":            u.CreatedAt,
			"summary_email_enabled": u.SummaryEmailEnabled,
			"summary_hour_utc":      u.SummaryHourUTC,
			"is_admin":              u.IsAdmin,
		},
	})
}

// ---------- JWT helpers ----------

func (a *App) setAuthCookie(c *gin.Context, userID string, tokenVersion int) error {
	now := time.Now()
	claims := authClaims{
		UserID: userID,
		TV:     tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * 24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(a.cfg.Auth.JWTSecret))
	if err != nil {
		return err
	}

	cfg := &a.cfg.Auth
	c.SetCookie(cfg.CookieName, signed,
		int((30 * 24 * time.Hour).Seconds()),
		"/",
		cfg.CookieDomain,
		cfg.CookieSecure,
		true,
	)
	return nil
}

func (a *App) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := &a.cfg.Auth
		raw, err := c.Cookie(cfg.CookieName)
		if err != nil || raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		token, err := jwt.ParseWithClaims(raw, &authClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(*authClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			return
		}

		uid, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		var user User
		err = a.usersColl.FindOne(ctx, bson.M{"_id": uid}).Decode(&user)
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		if err != nil {
			log.Printf("find user in middleware: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		if claims.TV != user.TokenVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session revoked, please log in again"})
			return
		}

		c.Set("user", &user)
		c.Next()
	}
}

// ---------- Token helpers ----------

func generateTokenString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *App) createToken(ctx context.Context, userID primitive.ObjectID, tokenType string, ttl time.Duration) (*Token, error) {
	tokenStr, err := generateTokenString(32)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	t := &Token{
		UserID:    userID,
		Token:     tokenStr,
		Type:      tokenType,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		Used:      false,
	}
	res, err := a.tokensColl.InsertOne(ctx, t)
	if err != nil {
		return nil, err
	}
	t.ID = res.InsertedID.(primitive.ObjectID)
	return t, nil
}

// ---------- Magic link login ----------

func (a *App) handleMagicRequest(c *gin.Context) {
	var req magicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var user User
	err := a.usersColl.FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		// Don't leak existence
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"message": "If this email exists, a magic link has been sent.",
		})
		return
	}
	if err != nil {
		log.Printf("magic find user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	t, err := a.createToken(ctx, user.ID, "magic_login", 30*time.Minute)
	if err != nil {
		log.Printf("create magic token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	link := fmt.Sprintf("%s/magic?token=%s", a.cfg.App.BaseURL, t.Token)
	log.Printf("[Magic link] %s", link)

	subject := "Your RunGuard magic login link"
	text := fmt.Sprintf(
		"Hi,\n\nUse this link to sign in to RunGuard:\n\n%s\n\nThis link expires in 30 minutes.\nIf you didn't request it, you can ignore this email.\n\n– RunGuard",
		link,
	)

	if err := a.sendMail(user.Email, subject, text, ""); err != nil {
		log.Printf("send magic mail: %v", err)
		// For security, don't expose details.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send email"})
		return
	}

	// For now we can still include magic_link in response for dev,
	// or remove it if you prefer stricter behavior.
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "If this email exists, a magic link has been sent.",
		// "magic_link": link, // uncomment if you want dev convenience
	})
}

func (a *App) handleMagicConsume(c *gin.Context) {
	var req tokenConsumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var t Token
	err := a.tokensColl.FindOne(ctx, bson.M{
		"token": req.Token,
		"type":  "magic_login",
	}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}
	if err != nil {
		log.Printf("magic find token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	now := time.Now()
	if t.Used || now.After(t.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token expired or already used"})
		return
	}

	// mark used
	_, err = a.tokensColl.UpdateByID(ctx, t.ID, bson.M{
		"$set": bson.M{
			"used":    true,
			"used_at": now,
		},
	})
	if err != nil {
		log.Printf("update magic token: %v", err)
	}

	// Following a link sent to the address proves the user owns it.
	a.markEmailVerified(ctx, t.UserID)

	// load user
	var user User
	err = a.usersColl.FindOne(ctx, bson.M{"_id": t.UserID}).Decode(&user)
	if err != nil {
		log.Printf("magic find user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	if err := a.setAuthCookie(c, user.ID.Hex(), user.TokenVersion); err != nil {
		log.Printf("set cookie magic: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":    user.ID.Hex(),
			"email": user.Email,
		},
	})
}

// ---------- Password reset ----------

func (a *App) handlePasswordResetRequest(c *gin.Context) {
	var req resetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var user User
	err := a.usersColl.FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"message": "If this email exists, a reset link has been sent.",
		})
		return
	}
	if err != nil {
		log.Printf("reset find user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	t, err := a.createToken(ctx, user.ID, "password_reset", 30*time.Minute)
	if err != nil {
		log.Printf("create reset token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	link := fmt.Sprintf("%s/reset?token=%s", a.cfg.App.BaseURL, t.Token)
	log.Printf("[Password reset link] %s", link)

	subject := "Reset your RunGuard password"
	text := fmt.Sprintf(
		"Hi,\n\nYou requested a password reset for RunGuard.\nUse this link to set a new password:\n\n%s\n\nThis link expires in 30 minutes.\nIf you didn't request this, you can ignore this email.\n\n– RunGuard",
		link,
	)

	if err := a.sendMail(user.Email, subject, text, ""); err != nil {
		log.Printf("send reset mail: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "If this email exists, a reset link has been sent.",
		// "reset_link": link, // keep/comment as you like for dev
	})
}

func (a *App) handlePasswordResetConfirm(c *gin.Context) {
	var req resetConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": authPayloadError(err)})
		return
	}
	if err := validatePasswordComplexity(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var t Token
	err := a.tokensColl.FindOne(ctx, bson.M{
		"token": req.Token,
		"type":  "password_reset",
	}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}
	if err != nil {
		log.Printf("reset find token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	now := time.Now()
	if t.Used || now.After(t.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token expired or already used"})
		return
	}

	// hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash reset pwd: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// update user. Bumping token_version invalidates every session token
	// that existed before this password change (including one an attacker
	// might have obtained), not just the one used to request this reset.
	_, err = a.usersColl.UpdateByID(ctx, t.UserID, bson.M{
		"$set": bson.M{
			"password_hash": string(hash),
		},
		"$inc": bson.M{
			"token_version": 1,
		},
	})
	if err != nil {
		log.Printf("update user pwd: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	// The reset link went to the address, so it's verified now too.
	a.markEmailVerified(ctx, t.UserID)

	// mark token used
	_, err = a.tokensColl.UpdateByID(ctx, t.ID, bson.M{
		"$set": bson.M{
			"used":    true,
			"used_at": now,
		},
	})
	if err != nil {
		log.Printf("update reset token: %v", err)
	}

	// log the user in after reset
	var user User
	if err := a.usersColl.FindOne(ctx, bson.M{"_id": t.UserID}).Decode(&user); err == nil {
		if err := a.setAuthCookie(c, user.ID.Hex(), user.TokenVersion); err != nil {
			log.Printf("set cookie after reset: %v", err)
		}
		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"id":    user.ID.Hex(),
				"email": user.Email,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
