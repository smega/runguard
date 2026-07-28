package main

import (
	"log"
	"strings"
)

const (
	PlanFree  = "free"
	PlanPlus  = "plus"
	PlanScale = "scale"
)

type PlanFeatures struct {
	ID                   string
	Label                string
	MaxJobs              int
	MaxProjects          int
	AllowWebhooks        bool
	AllowJobEmailAlerts  bool
	AllowPublicLinks     bool
	AllowDailySummary    bool
	AllowExports         bool
	AllowMultipleProject bool
	AllowPDFExport       bool
}

// hardcodedFreeFallback is only used if config.yml's plans list is empty or
// missing the requested plan entirely (e.g. a bad deploy) — enforcement
// should never panic or silently allow everything just because config is
// incomplete. In normal operation this is never reached.
var hardcodedFreeFallback = PlanFeatures{ID: PlanFree, Label: "Free", MaxJobs: 1, MaxProjects: 1, AllowDailySummary: true}

// planFeaturesFromConfig converts a config.yml PlanConfig into the
// PlanFeatures shape used for enforcement. config.yml (cfg.Plans) is the
// single source of truth for plan limits/features — this used to be a
// separate hardcoded map (planMatrix) that could silently drift from what
// config.yml — and therefore the pricing/account pages — actually said.
func planFeaturesFromConfig(p PlanConfig) PlanFeatures {
	return PlanFeatures{
		ID:                   p.ID,
		Label:                p.Label,
		MaxJobs:              p.MaxJobs,
		MaxProjects:          p.MaxProjects,
		AllowWebhooks:        p.AllowWebhooks,
		AllowJobEmailAlerts:  p.AllowEmailAlerts,
		AllowPublicLinks:     p.AllowPublicLinks,
		AllowDailySummary:    p.AllowDailySummary,
		AllowExports:         p.AllowExports,
		AllowMultipleProject: p.AllowMultipleProject,
		AllowPDFExport:       p.AllowPDFExport,
	}
}

func (a *App) getPlanFeatures(planID string) PlanFeatures {
	if planID == "" {
		planID = PlanFree
	}
	for _, p := range a.cfg.Plans {
		if p.ID == planID {
			return planFeaturesFromConfig(p)
		}
	}
	for _, p := range a.cfg.Plans {
		if p.ID == PlanFree {
			return planFeaturesFromConfig(p)
		}
	}
	log.Printf("plan %q not found in config.yml and no free plan configured — using built-in fallback limits", planID)
	return hardcodedFreeFallback
}

func (a *App) getPlanByID(id string) PlanConfig {
	if id == "" {
		id = "free"
	}
	for _, p := range a.cfg.Plans {
		if p.ID == id {
			return p
		}
	}
	if len(a.cfg.Plans) > 0 {
		return a.cfg.Plans[0]
	}
	return PlanConfig{ID: "free", Label: "Free", MaxJobs: 1, MaxProjects: 1, AllowEmailAlerts: false, AllowPublicLinks: false, AllowExports: false}
}

func (a *App) getUserPlanFeatures(u *User) PlanFeatures {
	base := a.getPlanFeatures(u.Plan)
	if u != nil && u.IsAdmin {
		base.AllowWebhooks = true
		base.AllowJobEmailAlerts = true
		base.AllowPublicLinks = true
		base.AllowDailySummary = true
		base.AllowExports = true
		if base.MaxJobs < 9999 {
			base.MaxJobs = 9999
		}
		if base.MaxProjects < 9999 {
			base.MaxProjects = 9999
		}
	}
	return base
}

func (a *App) getBillingPlan(id string) *BillingPlanConfig {
	for _, p := range a.cfg.Billing.Plans {
		if p.PlanID == id {
			cp := p
			return &cp
		}
	}
	return nil
}

func (a *App) getBillingPlanIDByPriceID(priceID string) string {
	priceID = strings.TrimSpace(priceID)
	if priceID == "" {
		return ""
	}
	for _, p := range a.cfg.Billing.Plans {
		if strings.TrimSpace(p.StripePriceID) == priceID {
			return p.PlanID
		}
	}
	return ""
}
