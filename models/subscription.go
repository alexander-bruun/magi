package models

import (
	"database/sql"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3/log"
)

// Subscription represents a user subscription
type Subscription struct {
	ID                   int       `json:"id"`
	Username             string    `json:"username"`
	StripeCustomerID     string    `json:"stripe_customer_id"`
	StripeSubscriptionID string    `json:"stripe_subscription_id"`
	Status               string    `json:"status"` // active, canceled, past_due, etc.
	CurrentPeriodStart   time.Time `json:"current_period_start"`
	CurrentPeriodEnd     time.Time `json:"current_period_end"`
	CancelAtPeriodEnd    bool      `json:"cancel_at_period_end"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// SubscriptionPlan represents available subscription plans
type SubscriptionPlan struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Duration    int     `json:"duration_months"` // in months
	Price       float64 `json:"price"`           // in cents
	Description string  `json:"description"`
	Savings     string  `json:"savings,omitempty"` // e.g., "Save 20%"
}

// GetSubscriptionPlans returns available subscription plans
func GetSubscriptionPlans() []SubscriptionPlan {
	return []SubscriptionPlan{
		{
			ID:          "1_month",
			Name:        "1 Month",
			Duration:    1,
			Price:       499, // $4.99
			Description: "Access premium features for 1 month",
		},
		{
			ID:          "3_months",
			Name:        "3 Months",
			Duration:    3,
			Price:       1299, // $12.99
			Description: "Access premium features for 3 months",
			Savings:     "Save 13%",
		},
		{
			ID:          "6_months",
			Name:        "6 Months",
			Duration:    6,
			Price:       2399, // $23.99
			Description: "Access premium features for 6 months",
			Savings:     "Save 20%",
		},
		{
			ID:          "1_year",
			Name:        "1 Year",
			Duration:    12,
			Price:       3999, // $39.99
			Description: "Access premium features for 1 year",
			Savings:     "Save 33%",
		},
	}
}

// GetUserSubscription retrieves the active subscription for a user
func GetUserSubscription(username string) (*Subscription, error) {
	query := `
	SELECT id, username, stripe_customer_id, stripe_subscription_id, status,
		   current_period_start, current_period_end, cancel_at_period_end,
		   created_at, updated_at
	FROM user_subscriptions
	WHERE username = ? AND status = 'active'
	ORDER BY created_at DESC
	LIMIT 1
	`

	var sub Subscription
	err := db.QueryRow(query, username).Scan(
		&sub.ID, &sub.Username, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
		&sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
		&sub.CancelAtPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No active subscription
		}
		return nil, err
	}

	return &sub, nil
}

// CreateSubscription creates a new subscription record
func CreateSubscription(sub *Subscription) error {
	query := `
	INSERT INTO user_subscriptions (
		username, stripe_customer_id, stripe_subscription_id, status,
		current_period_start, current_period_end, cancel_at_period_end,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = now
	}
	if sub.UpdatedAt.IsZero() {
		sub.UpdatedAt = now
	}

	result, err := db.Exec(query,
		sub.Username, sub.StripeCustomerID, sub.StripeSubscriptionID, sub.Status,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd,
		sub.CreatedAt, sub.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	sub.ID = int(id)

	return nil
}

// UpdateSubscription updates an existing subscription
func UpdateSubscription(sub *Subscription) error {
	query := `
	UPDATE user_subscriptions
	SET stripe_customer_id = ?, stripe_subscription_id = ?, status = ?,
		current_period_start = ?, current_period_end = ?, cancel_at_period_end = ?,
		updated_at = ?
	WHERE id = ?
	`

	sub.UpdatedAt = time.Now()

	_, err := db.Exec(query,
		sub.StripeCustomerID, sub.StripeSubscriptionID, sub.Status,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd,
		sub.UpdatedAt, sub.ID,
	)

	return err
}

// CancelSubscription marks a subscription as canceled
func CancelSubscription(username string) error {
	query := `
	UPDATE user_subscriptions
	SET status = 'canceled', cancel_at_period_end = true, updated_at = ?
	WHERE username = ? AND status = 'active'
	`

	_, err := db.Exec(query, time.Now(), username)
	return err
}

// GetSubscriptionByStripeID retrieves a subscription by Stripe subscription ID
func GetSubscriptionByStripeID(stripeSubscriptionID string) (*Subscription, error) {
	query := `
	SELECT id, username, stripe_customer_id, stripe_subscription_id, status,
		   current_period_start, current_period_end, cancel_at_period_end,
		   created_at, updated_at
	FROM user_subscriptions
	WHERE stripe_subscription_id = ?
	`

	var sub Subscription
	err := db.QueryRow(query, stripeSubscriptionID).Scan(
		&sub.ID, &sub.Username, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
		&sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
		&sub.CancelAtPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("subscription not found")
		}
		return nil, err
	}

	return &sub, nil
}

// UpdateUserRoleToPremium updates a user's role to premium
func UpdateUserRoleToPremium(username string) error {
	query := `UPDATE users SET role = 'premium' WHERE username = ?`
	_, err := db.Exec(query, username)
	if err != nil {
		log.Errorf("Failed to update user role to premium for %s: %v", username, err)
		return err
	}
	return nil
}

// UpdateUserRoleToReader updates a user's role back to reader
func UpdateUserRoleToReader(username string) error {
	query := `UPDATE users SET role = 'reader' WHERE username = ?`
	_, err := db.Exec(query, username)
	if err != nil {
		log.Errorf("Failed to update user role to reader for %s: %v", username, err)
		return err
	}
	return nil
}

// GetExpiredSubscriptions returns usernames of users with expired active subscriptions
func GetExpiredSubscriptions() ([]string, error) {
	query := `
		SELECT u.username
		FROM users u
		JOIN user_subscriptions us ON u.username = us.username
		WHERE us.status = 'active'
		AND us.current_period_end < datetime('now')
		AND NOT us.cancel_at_period_end
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		usernames = append(usernames, username)
	}

	return usernames, rows.Err()
}

// ExpireSubscription marks a subscription as expired
func ExpireSubscription(username string) error {
	query := `
		UPDATE user_subscriptions
		SET status = 'expired', updated_at = datetime('now')
		WHERE username = ? AND status = 'active'
	`

	_, err := db.Exec(query, username)
	return err
}
