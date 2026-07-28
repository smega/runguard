package main

import (
	"gopkg.in/yaml.v3"
	"os"
)

type BillingPlanConfig struct {
	PlanID          string `yaml:"plan_id" json:"plan_id"`
	PriceMonthlyGBP int    `yaml:"price_monthly_gbp" json:"price_monthly_gbp"`
	StripePriceID   string `yaml:"stripe_price_id" json:"stripe_price_id"`
	CheckoutURL     string `yaml:"checkout_url" json:"checkout_url"`
	PortalURL       string `yaml:"portal_url" json:"portal_url"`
	Highlight       bool   `yaml:"highlight" json:"highlight"`
}

// PlanConfig is the single source of truth for plan limits/features. It's
// used both to enforce limits (job/project creation, feature gating) and to
// describe plans to the frontend (pricing page, account page) — previously
// enforcement instead read from a separate hardcoded matrix in plans.go
// that could silently drift from what config.yml (and therefore the
// pricing page) said, since nothing kept the two in sync.
type PlanConfig struct {
	ID                   string   `yaml:"id" json:"id"`
	Label                string   `yaml:"label" json:"label"`
	MaxJobs              int      `yaml:"max_jobs" json:"max_jobs"`
	MaxProjects          int      `yaml:"max_projects" json:"max_projects"`
	AllowWebhooks        bool     `yaml:"allow_webhooks" json:"allow_webhooks"`
	AllowEmailAlerts     bool     `yaml:"allow_email_alerts" json:"allow_email_alerts"`
	AllowPublicLinks     bool     `yaml:"allow_public_links" json:"allow_public_links"`
	AllowDailySummary    bool     `yaml:"allow_daily_summary" json:"allow_daily_summary"`
	AllowExports         bool     `yaml:"allow_exports" json:"allow_exports"`
	AllowMultipleProject bool     `yaml:"allow_multiple_projects" json:"allow_multiple_projects"`
	AllowPDFExport       bool     `yaml:"allow_pdf_export" json:"allow_pdf_export"`
	Bullets              []string `yaml:"bullets" json:"bullets"`
}
type Config struct {
	Server struct {
		Addr           string `yaml:"addr"`
		FrontendOrigin string `yaml:"frontend_origin"`
		// TrustedProxies lists the IPs/CIDRs of reverse proxies/load
		// balancers in front of this server (e.g. your nginx box, a CDN's
		// edge IPs). Only those addresses' X-Forwarded-For gets honored
		// for client IP detection (used by the rate limiter and request
		// logging) — otherwise anyone could spoof that header to dodge
		// per-IP rate limits. Leave empty if the backend is reachable
		// directly (no proxy in front of it).
		TrustedProxies []string `yaml:"trusted_proxies"`
	} `yaml:"server"`
	PublicBaseURL string `yaml:"public_base_url"`
	Mongo         struct {
		URI      string `yaml:"uri"`
		Database string `yaml:"database"`
	} `yaml:"mongo"`
	Auth struct {
		JWTSecret    string `yaml:"jwt_secret"`
		CookieName   string `yaml:"cookie_name"`
		CookieSecure bool   `yaml:"cookie_secure"`
		CookieDomain string `yaml:"cookie_domain"`
	} `yaml:"auth"`
	Admin struct {
		// Emails in this list are automatically (and idempotently)
		// granted admin on every backend startup. This is the supported
		// way to create/recover an admin account — no more manual
		// mongosh commands. Safe to leave an email here permanently: it
		// only ever sets is_admin, never clears it.
		BootstrapEmails []string `yaml:"bootstrap_emails"`
	} `yaml:"admin"`
	App struct {
		BaseURL string `yaml:"base_url"`
	} `yaml:"app"`
	Mailgun struct {
		Enabled bool   `yaml:"enabled"`
		Domain  string `yaml:"domain"`
		BaseURL string `yaml:"base_url"`
		APIKey  string `yaml:"api_key"`
		From    string `yaml:"from"`
	} `yaml:"mailgun"`
	Alerts struct {
		Enabled              bool `yaml:"enabled"`
		CheckIntervalSeconds int  `yaml:"check_interval_seconds"`
		MinLateSeconds       int  `yaml:"min_late_seconds"`
		CooldownSeconds      int  `yaml:"cooldown_seconds"`
	} `yaml:"alerts"`
	CLI struct {
		DownloadBase   string `yaml:"download_base" json:"download_base"`
		Stable         string `yaml:"stable" json:"stable"`
		RC             string `yaml:"rc" json:"rc"`
		Nightly        string `yaml:"nightly" json:"nightly"`
		DefaultChannel string `yaml:"default_channel" json:"default_channel"`
	} `yaml:"cli" json:"cli"`
	Billing struct {
		Provider        string              `yaml:"provider" json:"provider"`
		TrialDays       int                 `yaml:"trial_days" json:"trial_days"`
		StripeSecretKey string              `yaml:"stripe_secret_key" json:"-"`
		WebhookSecret   string              `yaml:"webhook_secret" json:"-"`
		SuccessURL      string              `yaml:"success_url" json:"success_url"`
		CancelURL       string              `yaml:"cancel_url" json:"cancel_url"`
		PortalReturnURL string              `yaml:"portal_return_url" json:"portal_return_url"`
		Plans           []BillingPlanConfig `yaml:"plans" json:"plans"`
	} `yaml:"billing" json:"billing"`
	Plans []PlanConfig `yaml:"plans" json:"plans"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	if cfg.Billing.Provider == "" {
		cfg.Billing.Provider = "stripe"
	}
	if cfg.Billing.TrialDays <= 0 {
		cfg.Billing.TrialDays = 14
	}
	if cfg.CLI.DownloadBase == "" {
		cfg.CLI.DownloadBase = "https://runguard-download.smega.eu"
	}
	if cfg.CLI.DefaultChannel == "" {
		cfg.CLI.DefaultChannel = "latest"
	}
	return &cfg, nil
}
