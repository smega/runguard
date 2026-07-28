package main

import "strings"

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

var planMatrix = map[string]PlanFeatures{
	PlanFree:  {ID: PlanFree, Label: "Free", MaxJobs: 1, MaxProjects: 1, AllowWebhooks: true, AllowJobEmailAlerts: false, AllowPublicLinks: false, AllowDailySummary: true, AllowExports: false, AllowMultipleProject: false, AllowPDFExport: false},
	PlanPlus:  {ID: PlanPlus, Label: "Plus", MaxJobs: 5, MaxProjects: 3, AllowWebhooks: true, AllowJobEmailAlerts: true, AllowPublicLinks: true, AllowDailySummary: true, AllowExports: false, AllowMultipleProject: true, AllowPDFExport: false},
	PlanScale: {ID: PlanScale, Label: "Scale", MaxJobs: 20, MaxProjects: 10, AllowWebhooks: true, AllowJobEmailAlerts: true, AllowPublicLinks: true, AllowDailySummary: true, AllowExports: true, AllowMultipleProject: true, AllowPDFExport: true},
}

func getPlanFeatures(planID string) PlanFeatures {
	if planID == "" {
		planID = PlanFree
	}
	if p, ok := planMatrix[planID]; ok {
		return p
	}
	return planMatrix[PlanFree]
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
	base := getPlanFeatures(u.Plan)
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
