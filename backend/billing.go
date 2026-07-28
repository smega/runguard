package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	stripe "github.com/stripe/stripe-go/v83"
	billingportalsession "github.com/stripe/stripe-go/v83/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v83/checkout/session"
	"github.com/stripe/stripe-go/v83/webhook"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type publicPricingResponse struct {
	Provider  string              `json:"provider"`
	TrialDays int                 `json:"trial_days"`
	Plans     []PlanConfig        `json:"plans"`
	Billing   []BillingPlanConfig `json:"billing"`
}

type checkoutRequest struct {
	PlanID string `json:"plan_id" binding:"required"`
}

type billingResponse struct {
	Provider            string              `json:"provider"`
	TrialDays           int                 `json:"trial_days"`
	CurrentPlanID       string              `json:"current_plan_id"`
	Plans               []PlanConfig        `json:"plans"`
	Billing             []BillingPlanConfig `json:"billing"`
	CurrentCheckout     *BillingPlanConfig  `json:"current_checkout,omitempty"`
	BillingStatus       string              `json:"billing_status"`
	BillingCustomerID   string              `json:"billing_customer_id,omitempty"`
	BillingSubscription string              `json:"billing_subscription_id,omitempty"`
	TrialEndsAt         *time.Time          `json:"trial_ends_at,omitempty"`
	PaidPlanActivatedAt *time.Time          `json:"paid_plan_activated_at,omitempty"`
}

type stripeCheckoutSessionEvent struct {
	ID                string            `json:"id"`
	Customer          string            `json:"customer"`
	CustomerEmail     string            `json:"customer_email"`
	Subscription      string            `json:"subscription"`
	ClientReferenceID string            `json:"client_reference_id"`
	Metadata          map[string]string `json:"metadata"`
	PaymentStatus     string            `json:"payment_status"`
	Status            string            `json:"status"`
}

type stripeSubscriptionPrice struct {
	ID string `json:"id"`
}

type stripeSubscriptionItem struct {
	Price stripeSubscriptionPrice `json:"price"`
}

type stripeSubscriptionItems struct {
	Data []stripeSubscriptionItem `json:"data"`
}

type stripeSubscriptionEvent struct {
	ID       string                  `json:"id"`
	Customer string                  `json:"customer"`
	Status   string                  `json:"status"`
	Metadata map[string]string       `json:"metadata"`
	Items    stripeSubscriptionItems `json:"items"`
}

type stripeInvoiceEvent struct {
	ID            string            `json:"id"`
	Customer      string            `json:"customer"`
	CustomerEmail string            `json:"customer_email"`
	Subscription  string            `json:"subscription"`
	Status        string            `json:"status"`
	Metadata      map[string]string `json:"metadata"`
}

func effectiveBillingStatus(u *User) string {
	if u == nil {
		return "inactive"
	}
	if u.IsAdmin {
		return "active"
	}
	if u.BillingStatus != "" {
		return u.BillingStatus
	}
	if u.TrialEndsAt != nil && u.TrialEndsAt.After(time.Now().UTC()) {
		return "trialing"
	}
	if u.Plan == PlanFree || u.Plan == "" {
		return "free"
	}
	return "inactive"
}

// RunTrialExpiryLoop periodically flips billing_status from "trialing" to
// "trial_expired" once trial_ends_at has passed for anyone who never
// actually subscribed. Without this, billing_status is set to "trialing" at
// signup and — since a Stripe webhook is the only other thing that ever
// changes it — stays "trialing" forever if the user never pays, so the
// account page and admin panel show an indefinite trial instead of the
// trial having actually ended.
func (a *App) RunTrialExpiryLoop() {
	const interval = 1 * time.Hour
	log.Printf("[trial] starting trial-expiry loop (interval=%s)", interval)

	// Run once immediately on startup, then on the ticker.
	a.expireStaleTrials(context.Background())

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		a.expireStaleTrials(context.Background())
	}
}

func (a *App) expireStaleTrials(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	now := time.Now().UTC()
	res, err := a.usersColl.UpdateMany(ctx,
		bson.M{
			"billing_status": "trialing",
			"trial_ends_at":  bson.M{"$lt": now},
			// billing_subscription_id is stored with omitempty, so an
			// unconverted trial user may have the field missing entirely
			// rather than present-and-empty — match both.
			"$or": []bson.M{
				{"billing_subscription_id": bson.M{"$exists": false}},
				{"billing_subscription_id": ""},
			},
		},
		bson.M{"$set": bson.M{"billing_status": "trial_expired"}},
	)
	if err != nil {
		log.Printf("[trial] expire stale trials: %v", err)
		return
	}
	if res.ModifiedCount > 0 {
		log.Printf("[trial] marked %d expired trial(s) as trial_expired", res.ModifiedCount)
	}
}

func (a *App) handlePublicPricing(c *gin.Context) {
	c.JSON(http.StatusOK, publicPricingResponse{Provider: a.cfg.Billing.Provider, TrialDays: a.cfg.Billing.TrialDays, Plans: a.cfg.Plans, Billing: a.cfg.Billing.Plans})
}

func (a *App) handleGetBilling(c *gin.Context) {
	u := c.MustGet("user").(*User)
	c.JSON(http.StatusOK, billingResponse{Provider: a.cfg.Billing.Provider, TrialDays: a.cfg.Billing.TrialDays, CurrentPlanID: u.Plan, Plans: a.cfg.Plans, Billing: a.cfg.Billing.Plans, CurrentCheckout: a.getBillingPlan(u.Plan), BillingStatus: effectiveBillingStatus(u), BillingCustomerID: u.BillingCustomerID, BillingSubscription: u.BillingSubscriptionID, TrialEndsAt: u.TrialEndsAt, PaidPlanActivatedAt: u.PaidPlanActivatedAt})
}

func (a *App) stripeConfigured() error {
	if strings.TrimSpace(a.cfg.Billing.Provider) != "stripe" {
		return fmt.Errorf("billing provider is not stripe")
	}
	if strings.TrimSpace(a.cfg.Billing.StripeSecretKey) == "" {
		return fmt.Errorf("stripe secret key is not configured")
	}
	return nil
}

func (a *App) stripeSetKey() {
	stripe.Key = strings.TrimSpace(a.cfg.Billing.StripeSecretKey)
}

func (a *App) handleCreateCheckoutLink(c *gin.Context) {
	u := c.MustGet("user").(*User)
	var req checkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.PlanID == "" || req.PlanID == PlanFree {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please select a paid plan"})
		return
	}
	if err := a.stripeConfigured(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bp := a.getBillingPlan(req.PlanID)
	if bp == nil || strings.TrimSpace(bp.StripePriceID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stripe price id not configured for that plan"})
		return
	}

	a.stripeSetKey()
	successURL := strings.TrimSpace(a.cfg.Billing.SuccessURL)
	cancelURL := strings.TrimSpace(a.cfg.Billing.CancelURL)
	if successURL == "" || cancelURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "billing success/cancel URLs are not configured"})
		return
	}

	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		ClientReferenceID: stripe.String(u.ID.Hex()),
		CustomerEmail:     stripe.String(u.Email),
		Metadata: map[string]string{
			"user_id": u.ID.Hex(),
			"email":   strings.ToLower(strings.TrimSpace(u.Email)),
			"plan_id": req.PlanID,
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			Price:    stripe.String(bp.StripePriceID),
			Quantity: stripe.Int64(1),
		}},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"user_id": u.ID.Hex(),
				"email":   strings.ToLower(strings.TrimSpace(u.Email)),
				"plan_id": req.PlanID,
			},
		},
	}
	if strings.TrimSpace(u.BillingCustomerID) != "" {
		params.Customer = stripe.String(strings.TrimSpace(u.BillingCustomerID))
		params.CustomerEmail = nil
	}
	if u.TrialEndsAt != nil && u.TrialEndsAt.After(time.Now().UTC().Add(1*time.Hour)) {
		params.SubscriptionData.TrialEnd = stripe.Int64(u.TrialEndsAt.UTC().Unix())
	}

	sess, err := checkoutsession.New(params)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create stripe checkout session", "detail": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	_, _ = a.usersColl.UpdateByID(ctx, u.ID, bson.M{"$set": bson.M{"billing_provider": a.cfg.Billing.Provider, "billing_status": "checkout_pending", "billing_checkout_plan_id": req.PlanID, "billing_last_checkout_at": time.Now().UTC()}})
	c.JSON(http.StatusOK, gin.H{"url": sess.URL, "plan_id": req.PlanID, "session_id": sess.ID})
}

func (a *App) handleCreatePortalLink(c *gin.Context) {
	u := c.MustGet("user").(*User)
	if err := a.stripeConfigured(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(u.BillingCustomerID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no stripe customer is linked to this account yet"})
		return
	}
	returnURL := strings.TrimSpace(a.cfg.Billing.PortalReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(a.cfg.Billing.SuccessURL)
	}
	if returnURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "billing portal return url is not configured"})
		return
	}

	a.stripeSetKey()
	s, err := billingportalsession.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(strings.TrimSpace(u.BillingCustomerID)),
		ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create stripe billing portal session", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": s.URL})
}

func (a *App) handleBillingStripeWebhook(c *gin.Context) {
	if strings.TrimSpace(a.cfg.Billing.WebhookSecret) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stripe webhook secret is not configured"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}
	event, err := webhook.ConstructEventWithOptions(
		raw,
		c.GetHeader("Stripe-Signature"),
		strings.TrimSpace(a.cfg.Billing.WebhookSecret),
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stripe webhook signature", "detail": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	switch event.Type {
	case "checkout.session.completed":
		var payload stripeCheckoutSessionEvent
		if err := json.Unmarshal(event.Data.Raw, &payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse checkout session payload"})
			return
		}
		user, err := a.findUserForStripeEvent(ctx, payload.ClientReferenceID, payload.Customer, payload.CustomerEmail, payload.Metadata)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		planID := strings.TrimSpace(payload.Metadata["plan_id"])
		if planID == "" {
			planID = user.Plan
		}
		if planID == "" {
			planID = PlanFree
		}
		status := "active"
		if strings.EqualFold(payload.PaymentStatus, "unpaid") {
			status = "incomplete"
		}
		if err := a.applyStripeBillingState(ctx, user, stripeBillingState{PlanID: planID, Status: status, CustomerID: payload.Customer, SubscriptionID: payload.Subscription, EventType: string(event.Type), MarkPaidActivated: status == "active"}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update billing state"})
			return
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var payload stripeSubscriptionEvent
		if err := json.Unmarshal(event.Data.Raw, &payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse subscription payload"})
			return
		}
		user, err := a.findUserForStripeEvent(ctx, payload.Metadata["user_id"], payload.Customer, payload.Metadata["email"], payload.Metadata)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		planID := strings.TrimSpace(payload.Metadata["plan_id"])
		if planID == "" {
			planID = a.getBillingPlanIDByPriceID(firstStripePriceID(payload.Items))
		}
		state := stripeBillingState{PlanID: planID, Status: normalizeStripeSubscriptionStatus(payload.Status), CustomerID: payload.Customer, SubscriptionID: payload.ID, EventType: string(event.Type), MarkPaidActivated: payload.Status == "active" || payload.Status == "trialing"}
		if err := a.applyStripeBillingState(ctx, user, state); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update billing state"})
			return
		}
	case "invoice.payment_failed":
		var payload stripeInvoiceEvent
		if err := json.Unmarshal(event.Data.Raw, &payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse invoice payload"})
			return
		}
		user, err := a.findUserForStripeEvent(ctx, payload.Metadata["user_id"], payload.Customer, payload.CustomerEmail, payload.Metadata)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := a.applyStripeBillingState(ctx, user, stripeBillingState{PlanID: user.Plan, Status: "past_due", CustomerID: payload.Customer, SubscriptionID: payload.Subscription, EventType: string(event.Type), MarkPaidActivated: false}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update billing state"})
			return
		}
	case "invoice.paid":
		var payload stripeInvoiceEvent
		if err := json.Unmarshal(event.Data.Raw, &payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse invoice payload"})
			return
		}
		user, err := a.findUserForStripeEvent(ctx, payload.Metadata["user_id"], payload.Customer, payload.CustomerEmail, payload.Metadata)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := a.applyStripeBillingState(ctx, user, stripeBillingState{PlanID: user.Plan, Status: "active", CustomerID: payload.Customer, SubscriptionID: payload.Subscription, EventType: string(event.Type), MarkPaidActivated: true}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update billing state"})
			return
		}
	default:
		c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": true, "event_type": string(event.Type)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "event_type": string(event.Type)})
}

type stripeBillingState struct {
	PlanID            string
	Status            string
	CustomerID        string
	SubscriptionID    string
	EventType         string
	MarkPaidActivated bool
}

func (a *App) findUserForStripeEvent(ctx context.Context, userID, customerID, email string, metadata map[string]string) (*User, error) {
	if userID == "" && metadata != nil {
		userID = metadata["user_id"]
	}
	if email == "" && metadata != nil {
		email = metadata["email"]
	}
	var filter bson.M
	if strings.TrimSpace(userID) != "" {
		oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(userID))
		if err != nil {
			return nil, err
		}
		filter = bson.M{"_id": oid}
	} else if strings.TrimSpace(customerID) != "" {
		filter = bson.M{"billing_customer_id": strings.TrimSpace(customerID)}
	} else if strings.TrimSpace(email) != "" {
		filter = bson.M{"email": strings.ToLower(strings.TrimSpace(email))}
	} else {
		return nil, fmt.Errorf("unable to map stripe event to user")
	}
	var user User
	if err := a.usersColl.FindOne(ctx, filter).Decode(&user); err != nil {
		return nil, fmt.Errorf("user not found for stripe event")
	}
	return &user, nil
}

func (a *App) applyStripeBillingState(ctx context.Context, user *User, state stripeBillingState) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	now := time.Now().UTC()
	update := bson.M{
		"billing_provider": strings.TrimSpace(a.cfg.Billing.Provider),
		"billing_status":   strings.TrimSpace(state.Status),
	}
	if strings.TrimSpace(state.CustomerID) != "" {
		update["billing_customer_id"] = strings.TrimSpace(state.CustomerID)
	}
	if strings.TrimSpace(state.SubscriptionID) != "" {
		update["billing_subscription_id"] = strings.TrimSpace(state.SubscriptionID)
	}
	if strings.TrimSpace(state.PlanID) != "" {
		update["plan"] = strings.TrimSpace(state.PlanID)
	}
	if state.MarkPaidActivated {
		update["paid_plan_activated_at"] = now
	}
	update["billing_last_event_type"] = state.EventType
	update["billing_last_event_at"] = now

	switch state.EventType {
	case "customer.subscription.deleted":
		update["plan"] = PlanFree
		update["billing_status"] = "cancelled"
		update["billing_subscription_id"] = ""
	}
	_, err := a.usersColl.UpdateByID(ctx, user.ID, bson.M{"$set": update, "$unset": bson.M{"billing_checkout_plan_id": "", "billing_last_checkout_at": ""}})
	return err
}

func normalizeStripeSubscriptionStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "trialing", "active", "past_due", "canceled", "cancelled", "incomplete", "incomplete_expired", "unpaid", "paused":
		if status == "canceled" {
			return "cancelled"
		}
		return strings.TrimSpace(strings.ToLower(status))
	default:
		return "inactive"
	}
}

func firstStripePriceID(items stripeSubscriptionItems) string {
	if len(items.Data) == 0 {
		return ""
	}
	return strings.TrimSpace(items.Data[0].Price.ID)
}
