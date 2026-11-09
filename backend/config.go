package main

import (
	"gopkg.in/yaml.v3"
	"os"
)

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
	// Limits caps how many jobs/projects a non-admin user can create. 0 (or
	// omitted) means unlimited — RunGuard isn't monetized, so there's no
	// plan/tier system; this exists purely so the author's own hosted
	// instance can stay small (e.g. 3 jobs/user) while self-hosters run it
	// unlimited by just not setting it.
	Limits struct {
		MaxJobsPerUser     int `yaml:"max_jobs_per_user" json:"max_jobs_per_user"`
		MaxProjectsPerUser int `yaml:"max_projects_per_user" json:"max_projects_per_user"`
	} `yaml:"limits" json:"limits"`
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
	return &cfg, nil
}
