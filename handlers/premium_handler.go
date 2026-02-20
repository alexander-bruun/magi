package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/checkout/session"
	"github.com/stripe/stripe-go/v79/customer"
	"github.com/stripe/stripe-go/v79/webhook"
)

// HandlePremiumPage renders the premium subscription page
func HandlePremiumPage(c fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return SendInternalServerError(c, ErrConfigLoadFailed, err)
	}

	// Check if Stripe is enabled
	if !cfg.StripeEnabled {
		return handleView(c, views.Error("Premium subscriptions are currently unavailable."))
	}

	// Get current user
	userName := c.Locals("user_name")
	if userName == nil {
		log.Infof("Premium handler: redirecting to login, user not authenticated")
		return c.Redirect().To("/auth/login")
	}

	username := userName.(string)
	user, err := models.FindUserByUsername(username)
	if err != nil {
		return SendInternalServerError(c, ErrUserNotFound, err)
	}

	// Check if user already has premium
	if user.Role == "premium" {
		return handleView(c, views.Error("You already have premium access."))
	}

	plans := models.GetSubscriptionPlans()
	return handleView(c, views.PremiumPage(plans))
}

// HandleCreateCheckoutSession creates a Stripe checkout session
func HandleCreateCheckoutSession(c fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return SendInternalServerError(c, ErrConfigLoadFailed, err)
	}

	// Check if Stripe is enabled
	if !cfg.StripeEnabled {
		return c.Status(400).JSON(fiber.Map{"error": "Stripe payments are not enabled"})
	}

	// Set Stripe key
	stripe.Key = cfg.StripeSecretKey

	// Get current user
	userName := c.Locals("user_name")
	if userName == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	username := userName.(string)
	user, err := models.FindUserByUsername(username)
	if err != nil {
		return SendInternalServerError(c, ErrUserNotFound, err)
	}

	// Check if user already has premium
	if user.Role == "premium" {
		return c.Status(400).JSON(fiber.Map{"error": "User already has premium access"})
	}

	// Parse request
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// Validate plan
	plans := models.GetSubscriptionPlans()
	var selectedPlan *models.SubscriptionPlan
	for _, plan := range plans {
		if plan.ID == req.PlanID {
			selectedPlan = &plan
			break
		}
	}
	if selectedPlan == nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid plan selected"})
	}

	// Create or retrieve Stripe customer
	var stripeCustomer *stripe.Customer
	customerParams := &stripe.CustomerParams{
		Email: stripe.String(user.Username + "@placeholder.com"), // Using username as email placeholder
		Name:  stripe.String(user.Username),
		Metadata: map[string]string{
			"username": user.Username,
		},
	}

	customers := customer.List(&stripe.CustomerListParams{
		Email: stripe.String(user.Username + "@placeholder.com"),
	})
	if customers.Next() {
		stripeCustomer = customers.Customer()
	} else {
		customer, err := customer.New(customerParams)
		if err != nil {
			return SendInternalServerError(c, ErrStripeError, err)
		}
		stripeCustomer = customer
	}

	// Create checkout session
	sessionParams := &stripe.CheckoutSessionParams{
		Customer: stripe.String(stripeCustomer.ID),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(selectedPlan.Name + " Premium Subscription"),
						Description: stripe.String(selectedPlan.Description),
					},
					UnitAmount: stripe.Int64(int64(selectedPlan.Price)),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String("payment"),
		SuccessURL: stripe.String(fmt.Sprintf("%s/premium/success?session_id={CHECKOUT_SESSION_ID}", c.BaseURL())),
		CancelURL:  stripe.String(fmt.Sprintf("%s/premium", c.BaseURL())),
		Metadata: map[string]string{
			"username": user.Username,
			"plan_id":  selectedPlan.ID,
		},
	}

	sess, err := session.New(sessionParams)
	if err != nil {
		return SendInternalServerError(c, ErrStripeError, err)
	}

	return c.JSON(fiber.Map{
		"sessionId": sess.ID,
		"url":       sess.URL,
	})
}

// HandlePremiumSuccess handles successful payment
func HandlePremiumSuccess(c fiber.Ctx) error {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		return c.Status(400).SendString("Missing session ID")
	}

	cfg, err := models.GetAppConfig()
	if err != nil {
		return SendInternalServerError(c, ErrConfigLoadFailed, err)
	}

	// Set Stripe key
	stripe.Key = cfg.StripeSecretKey

	// Retrieve session
	sess, err := session.Get(sessionID, nil)
	if err != nil {
		return SendInternalServerError(c, ErrStripeError, err)
	}

	// Get user from metadata
	username := sess.Metadata["username"]
	if username == "" {
		return c.Status(400).SendString("Invalid session")
	}

	// Update user role to premium
	if err := models.UpdateUserRoleToPremium(username); err != nil {
		return SendInternalServerError(c, ErrDatabaseError, err)
	}

	// Create subscription record
	planID := sess.Metadata["plan_id"]
	plans := models.GetSubscriptionPlans()
	var selectedPlan *models.SubscriptionPlan
	for _, plan := range plans {
		if plan.ID == planID {
			selectedPlan = &plan
			break
		}
	}

	if selectedPlan != nil {
		now := time.Now()
		endDate := now.AddDate(0, selectedPlan.Duration, 0)

		sub := &models.Subscription{
			Username:             username,
			StripeCustomerID:     sess.Customer.ID,
			StripeSubscriptionID: "", // One-time payment, no recurring subscription
			Status:               "active",
			CurrentPeriodStart:   now,
			CurrentPeriodEnd:     endDate,
			CancelAtPeriodEnd:    false,
		}

		if err := models.CreateSubscription(sub); err != nil {
			// Log error but don't fail the request since user role was updated
			fmt.Printf("Failed to create subscription record: %v\n", err)
		}
	}

	return handleView(c, views.PremiumSuccess())
}

// HandleStripeWebhook handles Stripe webhook events
func HandleStripeWebhook(c fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return SendInternalServerError(c, ErrConfigLoadFailed, err)
	}

	if !cfg.StripeEnabled {
		return c.Status(400).SendString("Stripe not enabled")
	}

	// Get the raw body
	body := c.Body()

	// Get the signature from headers
	signature := c.Get("Stripe-Signature")
	if signature == "" {
		return c.Status(400).SendString("Missing signature")
	}

	// Verify webhook signature
	_, err = webhook.ConstructEvent(body, signature, cfg.StripeWebhookSecret)
	if err != nil {
		return c.Status(400).SendString("Invalid signature")
	}

	// Parse the event
	event, err := webhook.ConstructEvent(body, signature, cfg.StripeWebhookSecret)
	if err != nil {
		return c.Status(400).SendString("Invalid signature")
	}

	// Handle different event types
	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return c.Status(400).SendString("Invalid session data")
		}

		// Update subscription status if needed
		username := session.Metadata["username"]
		if username != "" {
			// User role should already be updated in success handler
			// This is just for additional processing if needed
		}
	}

	return c.SendString("OK")
}

// HandlePremiumCancel handles subscription cancellation
func HandlePremiumCancel(c fiber.Ctx) error {
	userName := c.Locals("user_name")
	if userName == nil {
		log.Infof("Premium cancel handler: redirecting to login, user not authenticated")
		return c.Redirect().To("/auth/login")
	}

	username := userName.(string)

	// Cancel subscription
	if err := models.CancelSubscription(username); err != nil {
		return SendInternalServerError(c, ErrDatabaseError, err)
	}

	// Update user role back to reader
	if err := models.UpdateUserRoleToReader(username); err != nil {
		return SendInternalServerError(c, ErrDatabaseError, err)
	}

	return c.Redirect().To("/account")
}

// HandleGetStripeConfig returns the Stripe publishable key for frontend use
func HandleGetStripeConfig(c fiber.Ctx) error {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to load config"})
	}

	if !cfg.StripeEnabled {
		return c.Status(400).JSON(fiber.Map{"error": "Stripe not enabled"})
	}

	return c.JSON(fiber.Map{
		"publishableKey": cfg.StripePublishableKey,
	})
}
