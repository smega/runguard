package main

import (
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type App struct {
	cfg *Config
	db  *mongo.Database

	usersColl       *mongo.Collection
	tokensColl      *mongo.Collection
	jobsColl        *mongo.Collection
	jobRunsColl     *mongo.Collection
	projectsColl    *mongo.Collection
	incidentsColl   *mongo.Collection
	webhookLogsColl *mongo.Collection

	webhookTemplatesColl *mongo.Collection
	adminAuditColl       *mongo.Collection
}

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`

	Plan                    string     `bson:"plan,omitempty" json:"plan"`
	SummaryEmailEnabled     bool       `bson:"summary_email_enabled,omitempty" json:"summary_email_enabled"`
	SummaryHourUTC          int        `bson:"summary_hour_utc,omitempty" json:"summary_hour_utc"`
	SummaryLastSentAt       *time.Time `bson:"summary_last_sent_at,omitempty" json:"summary_last_sent_at,omitempty"`
	WeeklySummaryEnabled    bool       `bson:"weekly_summary_enabled,omitempty" json:"weekly_summary_enabled"`
	WeeklySummaryLastSentAt *time.Time `bson:"weekly_summary_last_sent_at,omitempty" json:"weekly_summary_last_sent_at,omitempty"`
	BillingProvider         string     `bson:"billing_provider,omitempty" json:"billing_provider,omitempty"`
	BillingStatus           string     `bson:"billing_status,omitempty" json:"billing_status,omitempty"`
	BillingCustomerID       string     `bson:"billing_customer_id,omitempty" json:"billing_customer_id,omitempty"`
	BillingSubscriptionID   string     `bson:"billing_subscription_id,omitempty" json:"billing_subscription_id,omitempty"`
	BillingCheckoutPlanID   string     `bson:"billing_checkout_plan_id,omitempty" json:"billing_checkout_plan_id,omitempty"`
	BillingLastEventType    string     `bson:"billing_last_event_type,omitempty" json:"billing_last_event_type,omitempty"`
	BillingLastEventAt      *time.Time `bson:"billing_last_event_at,omitempty" json:"billing_last_event_at,omitempty"`
	BillingLastCheckoutAt   *time.Time `bson:"billing_last_checkout_at,omitempty" json:"billing_last_checkout_at,omitempty"`
	TrialEndsAt             *time.Time `bson:"trial_ends_at,omitempty" json:"trial_ends_at,omitempty"`
	PaidPlanActivatedAt     *time.Time `bson:"paid_plan_activated_at,omitempty" json:"paid_plan_activated_at,omitempty"`
	IsAdmin                 bool       `bson:"is_admin,omitempty" json:"is_admin"`

	// TokenVersion is embedded in issued JWTs. Bumping it (e.g. on password
	// change or an explicit "log out everywhere") immediately invalidates
	// every previously issued token, since authMiddleware rejects any token
	// whose version doesn't match the current value.
	TokenVersion int `bson:"token_version,omitempty" json:"-"`

	// AgentToken authenticates the runguard CLI for self-registration
	// (POST /api/agent/register), separate from the browser session cookie.
	// Generated lazily on first request, regenerable from the account page.
	AgentToken string `bson:"agent_token,omitempty" json:"-"`
}

// AdminAuditEntry records an administrative action for accountability —
// who did what, to whom, and when. Written by admin.go handlers.
type AdminAuditEntry struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	At          time.Time          `bson:"at" json:"at"`
	ActorID     primitive.ObjectID `bson:"actor_id" json:"actor_id"`
	ActorEmail  string             `bson:"actor_email" json:"actor_email"`
	Action      string             `bson:"action" json:"action"` // "promote" | "demote"
	TargetID    primitive.ObjectID `bson:"target_id" json:"target_id"`
	TargetEmail string             `bson:"target_email" json:"target_email"`
}

type Token struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Token     string             `bson:"token"`
	Type      string             `bson:"type"`
	CreatedAt time.Time          `bson:"created_at"`
	ExpiresAt time.Time          `bson:"expires_at"`
	UsedAt    *time.Time         `bson:"used_at,omitempty"`
	Used      bool               `bson:"used"`
}

type Project struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      primitive.ObjectID `bson:"user_id" json:"-"`
	Name        string             `bson:"name" json:"name"`
	Slug        string             `bson:"slug" json:"slug"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Environment string             `bson:"environment,omitempty" json:"environment,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time         `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

type Job struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID               primitive.ObjectID `bson:"user_id" json:"-"`
	Name                 string             `bson:"name" json:"name"`
	ProjectID            primitive.ObjectID `bson:"project_id,omitempty" json:"project_id,omitempty"`
	Key                  string             `bson:"key" json:"-"`
	ExpectedIntervalSecs int                `bson:"expected_interval_seconds" json:"expected_interval_seconds"`
	GraceSecs            int                `bson:"grace_seconds" json:"grace_seconds"`
	LastHeartbeatAt      *time.Time         `bson:"last_heartbeat_at,omitempty" json:"last_heartbeat_at,omitempty"`
	LastAlertSentAt      *time.Time         `bson:"last_alert_sent_at,omitempty" json:"last_alert_sent_at,omitempty"`
	LastRecoverySentAt   *time.Time         `bson:"last_recovery_sent_at,omitempty" json:"last_recovery_sent_at,omitempty"`
	CreatedAt            time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt            time.Time          `bson:"updated_at" json:"updated_at"`
	DeletedAt            *time.Time         `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	AlertEmailEnabled    bool               `bson:"alert_email_enabled,omitempty" json:"alert_email_enabled"`
	AlertWebhookEnabled  bool               `bson:"alert_webhook_enabled,omitempty" json:"alert_webhook_enabled"`
	AlertWebhookURLs     []string           `bson:"alert_webhook_urls,omitempty" json:"alert_webhook_urls,omitempty"`
	PublicToken          string             `bson:"publicToken,omitempty" json:"-"`
	PublicEnabled        bool               `bson:"publicEnabled,omitempty" json:"publicEnabled"`
	Severity             string             `bson:"severity,omitempty" json:"severity,omitempty"`
	MaintenanceUntil     *time.Time         `bson:"maintenance_until,omitempty" json:"maintenance_until,omitempty"`
	WebhookURL           string             `bson:"webhook_url,omitempty" json:"webhook_url,omitempty"`

	// CronExpr, when set, switches this job from plain interval+grace
	// checking to a standard 5-field cron expression ("0 3 * * *"). See
	// cron.go for the parser and jobLateness() for how the two modes are
	// reconciled. Empty means "use ExpectedIntervalSecs/GraceSecs" (the
	// original, still-default behavior).
	CronExpr string `bson:"cron_expr,omitempty" json:"cron_expr,omitempty"`
}

type Incident struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID          primitive.ObjectID `bson:"user_id" json:"user_id"`
	ProjectID       primitive.ObjectID `bson:"project_id,omitempty" json:"project_id,omitempty"`
	JobID           primitive.ObjectID `bson:"job_id" json:"job_id"`
	Kind            string             `bson:"kind" json:"kind"`
	State           string             `bson:"state" json:"state"`
	Title           string             `bson:"title" json:"title"`
	Summary         string             `bson:"summary,omitempty" json:"summary,omitempty"`
	OpenedAt        time.Time          `bson:"opened_at" json:"opened_at"`
	LastSeenAt      time.Time          `bson:"last_seen_at" json:"last_seen_at"`
	AcknowledgedAt  *time.Time         `bson:"acknowledged_at,omitempty" json:"acknowledged_at,omitempty"`
	ResolvedAt      *time.Time         `bson:"resolved_at,omitempty" json:"resolved_at,omitempty"`
	OccurrenceCount int                `bson:"occurrence_count" json:"occurrence_count"`
	LastRunStatus   string             `bson:"last_run_status,omitempty" json:"last_run_status,omitempty"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}

type WebhookTemplate struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Name      string             `bson:"name" json:"name"`
	URL       string             `bson:"url" json:"url"`
	Type      string             `bson:"type,omitempty" json:"type,omitempty"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

type JobRun struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	JobID        primitive.ObjectID `bson:"job_id" json:"job_id"`
	UserID       primitive.ObjectID `bson:"user_id" json:"user_id"`
	Ts           time.Time          `bson:"ts" json:"ts"`
	Status       string             `bson:"status" json:"status"`
	At           time.Time          `bson:"at" json:"at"`
	LateBySec    int64              `bson:"late_by_seconds,omitempty" json:"late_by_seconds,omitempty"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	ExitCode     *int               `bson:"exit_code,omitempty" json:"exit_code,omitempty"`
	DurationMs   int64              `bson:"duration_ms" json:"duration_ms"`
	Env          string             `bson:"env,omitempty" json:"env,omitempty"`
	Tags         []string           `bson:"tags,omitempty" json:"tags,omitempty"`
	AgentVersion string             `bson:"agent_version,omitempty" json:"agent_version,omitempty"`
	StdoutTail   *LogTail           `bson:"stdout_tail,omitempty" json:"stdout_tail,omitempty"`
	StderrTail   *LogTail           `bson:"stderr_tail,omitempty" json:"stderr_tail,omitempty"`
}

const (
	SeverityLow               = "low"
	SeverityNormal            = "normal"
	SeverityHigh              = "high"
	SeverityCritical          = "critical"
	IncidentStateOpen         = "open"
	IncidentStateAcknowledged = "acknowledged"
	IncidentStateResolved     = "resolved"
	IncidentKindLate          = "late"
	IncidentKindFail          = "fail"
)

func normalizeSeverity(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch s {
	case SeverityLow:
		return SeverityLow
	case SeverityHigh:
		return SeverityHigh
	case SeverityCritical, "crit":
		return SeverityCritical
	default:
		return SeverityNormal
	}
}

type HeartbeatRequest struct {
	JobID        string    `json:"job_id" binding:"required"`
	Ts           time.Time `json:"ts" binding:"required"`
	Status       string    `json:"status" binding:"required"`
	ExitCode     *int      `json:"exit_code,omitempty"`
	DurationMs   int64     `json:"duration_ms"`
	Env          string    `json:"env,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	AgentVersion string    `json:"agent_version,omitempty"`
	StdoutTail   string    `json:"stdout_tail,omitempty"`
	StderrTail   string    `json:"stderr_tail,omitempty"`
}
