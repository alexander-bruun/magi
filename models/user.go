package models

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v3/log"
	"golang.org/x/crypto/bcrypt"
)

// User represents the user table schema
type User struct {
	Username     string        `json:"username"`
	Password     string        `json:"password"`
	Role         string        `json:"role"`
	Banned       bool          `json:"banned"`
	Avatar       string        `json:"avatar,omitempty"`
	Email        string        `json:"email,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	Subscription *Subscription `json:"subscription,omitempty"` // Active subscription if any
}

// BannedIP represents a banned IP address
type BannedIP struct {
	ID        int        `json:"id"`
	IPAddress string     `json:"ip_address"`
	BannedAt  time.Time  `json:"banned_at"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// roleHierarchy defines the order of roles from lowest to highest.
var roleHierarchy = []string{"reader", "premium", "moderator", "admin"}

// GetUserRoleDistribution returns a map of role names to user counts
func GetUserRoleDistribution() (map[string]int, error) {
	query := `SELECT role, COUNT(*) as count FROM users GROUP BY role ORDER BY count DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	distribution := make(map[string]int)
	for rows.Next() {
		var role string
		var count int
		if err := rows.Scan(&role, &count); err != nil {
			return nil, err
		}
		distribution[role] = count
	}
	return distribution, nil
}

// GetUsers retrieves all Users from the database
func GetUsers() ([]User, error) {
	query := `
	SELECT username, password, role, banned, avatar, email, created_at
	FROM users
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Username, &user.Password, &user.Role, &user.Banned, &user.Avatar, &user.Email, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// GetUsersByRole retrieves all users with a specific role
func GetUsersByRole(role string) ([]User, error) {
	query := `
	SELECT username, password, role, banned, avatar, email, created_at
	FROM users
	WHERE role = ?
	`

	rows, err := db.Query(query, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Username, &user.Password, &user.Role, &user.Banned, &user.Avatar, &user.Email, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// UserSearchOptions holds options for user search and pagination
type UserSearchOptions struct {
	Filter   string
	Page     int
	PageSize int
}

// GetUsersWithOptions performs a flexible user search with SQL-level filtering and pagination
func GetUsersWithOptions(opts UserSearchOptions) ([]User, int64, error) {
	// Build WHERE clause
	var conditions []string
	var args []interface{}

	if opts.Filter != "" {
		conditions = append(conditions, "u.username LIKE ?")
		args = append(args, "%"+opts.Filter+"%")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total matching rows
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users u %s", where)
	var total int64
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	// Fetch paginated results with subscription data via LEFT JOIN
	dataQuery := fmt.Sprintf(`
		SELECT u.username, u.password, u.role, u.banned, u.avatar, u.email, u.created_at,
		       s.id, s.stripe_customer_id, s.stripe_subscription_id, s.status,
		       s.current_period_start, s.current_period_end, s.cancel_at_period_end,
		       s.created_at, s.updated_at
		FROM users u
		LEFT JOIN user_subscriptions s
		  ON s.username = u.username AND s.status = 'active'
		%s
		ORDER BY u.created_at DESC
		LIMIT ? OFFSET ?
	`, where)

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := opts.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	paginatedArgs := append(args, pageSize, offset)
	rows, err := db.Query(dataQuery, paginatedArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		var subID sql.NullInt64
		var subCustomerID, subSubscriptionID, subStatus sql.NullString
		var subPeriodStart, subPeriodEnd, subCreatedAt, subUpdatedAt sql.NullTime
		var subCancelAtPeriodEnd sql.NullBool

		if err := rows.Scan(
			&user.Username, &user.Password, &user.Role, &user.Banned, &user.Avatar, &user.Email, &user.CreatedAt,
			&subID, &subCustomerID, &subSubscriptionID, &subStatus,
			&subPeriodStart, &subPeriodEnd, &subCancelAtPeriodEnd,
			&subCreatedAt, &subUpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}

		if subID.Valid {
			user.Subscription = &Subscription{
				ID:                   int(subID.Int64),
				Username:             user.Username,
				StripeCustomerID:     subCustomerID.String,
				StripeSubscriptionID: subSubscriptionID.String,
				Status:               subStatus.String,
				CurrentPeriodStart:   subPeriodStart.Time,
				CurrentPeriodEnd:     subPeriodEnd.Time,
				CancelAtPeriodEnd:    subCancelAtPeriodEnd.Bool,
				CreatedAt:            subCreatedAt.Time,
				UpdatedAt:            subUpdatedAt.Time,
			}
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}

	return users, total, nil
}

// CreateUser creates a new user with hashed password and default role.
func CreateUser(username, password, email string) error {
	// Validate password strength
	if err := validatePassword(password); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := User{
		Username: username,
		Password: string(hashedPassword),
		Role:     "reader", // Default role
		Email:    email,
	}

	count, err := CountUsers()
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if count == 0 {
		log.Infof("No users have yet been registered, promoting '%s' to 'admin' role", user.Username)
		user.Role = "admin"
	}

	// Check if email is already in use
	var emailCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ?`, user.Email).Scan(&emailCount)
	if err != nil {
		return fmt.Errorf("failed to check email uniqueness: %w", err)
	}
	if emailCount > 0 {
		return fmt.Errorf("email already exists")
	}

	query := `
	INSERT INTO users (username, password, role, banned, avatar, email)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query, user.Username, user.Password, user.Role, user.Banned, user.Avatar, user.Email)
	if err != nil {
		return err
	}

	// Assign default 'all' permission to new user
	err = assignDefaultPermissionToUser(username)
	if err != nil {
		log.Warnf("Failed to assign default permission to user '%s': %v", username, err)
		// Don't fail user creation if permission assignment fails
	}

	return nil
}

// validatePassword checks that a password meets minimum security requirements:
// at least 8 characters, one uppercase letter, one lowercase letter, one digit, and one special character.
func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password too weak")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("password too weak")
	}
	return nil
}

// assignDefaultPermissionToUser assigns the 'all' wildcard permission to a user
func assignDefaultPermissionToUser(username string) error {
	// Find the 'all' permission
	query := `SELECT id FROM permissions WHERE name = 'all' AND is_wildcard = 1 LIMIT 1`
	var permID int64
	err := db.QueryRow(query).Scan(&permID)
	if err != nil {
		return fmt.Errorf("'all' permission not found: %w", err)
	}

	// Assign it to the user
	return AssignPermissionToUser(username, permID)
}

// FindUserByUsername retrieves a user by their username.
var FindUserByUsername = func(username string) (*User, error) {
	query := `
	SELECT username, password, role, banned, avatar, email
	FROM users
	WHERE username = ?
	`

	row := db.QueryRow(query, username)

	var user User
	err := row.Scan(&user.Username, &user.Password, &user.Role, &user.Banned, &user.Avatar, &user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No user found
		}
		return nil, err
	}

	return &user, nil
}

// UpdateUserRole updates the role of a user.
func UpdateUserRole(username, newRole string) error {
	return updateUserRoleWith(db, username, newRole)
}

// UpdateUserRoleTx updates the role of a user within a transaction.
func UpdateUserRoleTx(tx *sql.Tx, username, newRole string) error {
	return updateUserRoleWith(tx, username, newRole)
}

// updateUserRoleWith updates the role of a user using the given Executor.
func updateUserRoleWith(exec Executor, username, newRole string) error {
	if !isValidRole(newRole) {
		return errors.New("invalid role")
	}

	user, err := FindUserByUsername(username)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user '%s' not found", username)
	}

	user.Role = newRole
	query := `
	UPDATE users
	SET role = ?
	WHERE username = ?
	`

	_, err = exec.Exec(query, user.Role, username)
	if err != nil {
		return err
	}

	return nil
}

// ResetUserPassword resets a user's password to a new hashed password.
func ResetUserPassword(username, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := `
	UPDATE users
	SET password = ?
	WHERE username = ?
	`

	result, err := db.Exec(query, string(hashedPassword), username)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}

	return nil
}

// CountUsers returns the total number of users.
func CountUsers() (int64, error) {
	return CountRecords(`SELECT COUNT(*) FROM users`)
}

// isValidRole checks if the provided role is valid.
func isValidRole(role string) bool {
	return slices.Contains(roleHierarchy, role)
}

// getNextRole finds the next role in the hierarchy.
func getNextRole(currentRole string) (string, error) {
	for i, role := range roleHierarchy {
		if role == currentRole && i < len(roleHierarchy)-1 {
			return roleHierarchy[i+1], nil
		}
	}
	return "", errors.New("no higher role available")
}

// getPreviousRole finds the previous role in the hierarchy.
func getPreviousRole(currentRole string) (string, error) {
	for i, role := range roleHierarchy {
		if role == currentRole && i > 0 {
			return roleHierarchy[i-1], nil
		}
	}
	return "", errors.New("no lower role available")
}

// PromoteUser promotes a user to the next role in the hierarchy.
func PromoteUser(username string) error {
	user, err := FindUserByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to find user to promote: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user '%s' not found", username)
	}

	if user.Banned {
		return fmt.Errorf("user '%s' is banned and cannot be promoted", username)
	}

	nextRole, err := getNextRole(user.Role)
	if err != nil {
		return fmt.Errorf("failed to promote user: %w", err)
	}

	if err := UpdateUserRole(username, nextRole); err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	log.Infof("User '%s' has been promoted to '%s'", username, nextRole)
	return nil
}

// DemoteUser demotes a user to the previous role in the hierarchy.
func DemoteUser(username string) error {
	user, err := FindUserByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to find user to demote: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user '%s' not found", username)
	}

	if user.Banned {
		return fmt.Errorf("user '%s' is banned and cannot be demoted", username)
	}

	previousRole, err := getPreviousRole(user.Role)
	if err != nil {
		return fmt.Errorf("failed to demote user: %w", err)
	}

	if err := UpdateUserRole(username, previousRole); err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	log.Infof("User '%s' has been demoted to '%s'", username, previousRole)
	return nil
}

// BanUser bans a user by setting the Banned field to true.
func BanUser(username string) error {
	return banUserWith(db, username)
}

// BanUserTx bans a user by setting the Banned field to true within a transaction.
func BanUserTx(tx *sql.Tx, username string) error {
	return banUserWith(tx, username)
}

// banUserWith bans a user using the given Executor.
func banUserWith(exec Executor, username string) error {
	user, err := FindUserByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to find user to ban: %w", err)
	}

	if user.Banned {
		return fmt.Errorf("user '%s' is already banned", username)
	}

	user.Banned = true
	query := `
	UPDATE users
	SET banned = ?
	WHERE username = ?
	`

	_, err = exec.Exec(query, user.Banned, username)
	if err != nil {
		return err
	}

	log.Infof("User '%s' has been banned", username)
	return nil
}

// BanUserWithDemotion demotes a user to reader role and bans them atomically.
func BanUserWithDemotion(username string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := UpdateUserRoleTx(tx, username, "reader"); err != nil {
		return err
	}

	if err := BanUserTx(tx, username); err != nil {
		return err
	}

	return tx.Commit()
}

// UnbanUser unbans a user by setting the Banned field to false.
func UnbanUser(username string) error {
	user, err := FindUserByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to find user to unban: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user '%s' not found", username)
	}

	if !user.Banned {
		return fmt.Errorf("user '%s' is not banned", username)
	}

	user.Banned = false
	query := `
	UPDATE users
	SET banned = ?
	WHERE username = ?
	`

	_, err = db.Exec(query, user.Banned, username)
	if err != nil {
		return err
	}

	log.Infof("User '%s' has been unbanned", username)
	return nil
}

// BanIP adds an IP to the banned list with an optional duration in seconds.
// If durationSeconds <= 0, the ban is permanent.
func BanIP(ip, reason string, durationSeconds ...int) error {
	var expiresAt *time.Time
	if len(durationSeconds) > 0 && durationSeconds[0] > 0 {
		t := time.Now().Add(time.Duration(durationSeconds[0]) * time.Second)
		expiresAt = &t
	}
	_, err := db.Exec(`
		INSERT INTO banned_ips (ip_address, reason, expires_at)
		VALUES (?, ?, ?)
		ON CONFLICT(ip_address) DO UPDATE SET reason = excluded.reason, expires_at = excluded.expires_at, banned_at = CURRENT_TIMESTAMP
	`, ip, reason, expiresAt)
	if err != nil {
		log.Errorf("Failed to ban IP %s: %v", ip, err)
	}
	return err
}

// UnbanIP removes an IP from the banned list
func UnbanIP(ip string) error {
	_, err := db.Exec(`
		DELETE FROM banned_ips WHERE ip_address = ?
	`, ip)
	return err
}

// IsIPBanned checks if an IP is currently banned (respects expiry).
// Expired bans are cleaned up automatically.
func IsIPBanned(ip string) (bool, error) {
	// Clean up expired bans for this IP
	_, _ = db.Exec(`DELETE FROM banned_ips WHERE ip_address = ? AND expires_at IS NOT NULL AND expires_at <= ?`, ip, time.Now())

	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM banned_ips WHERE ip_address = ?
	`, ip).Scan(&count)
	if err != nil {
		log.Errorf("Failed to check if IP %s is banned: %v", ip, err)
		return false, err
	}
	banned := count > 0
	if banned {
		log.Infof("IP %s is banned", ip)
	}
	return banned, nil
}

// GetBannedIPs retrieves all currently active banned IPs (expired bans are cleaned up).
func GetBannedIPs() ([]BannedIP, error) {
	// Clean up all expired bans
	_, _ = db.Exec(`DELETE FROM banned_ips WHERE expires_at IS NOT NULL AND expires_at <= ?`, time.Now())

	rows, err := db.Query(`
		SELECT id, ip_address, banned_at, reason, expires_at
		FROM banned_ips
		ORDER BY banned_at DESC
	`)
	if err != nil {
		log.Errorf("Failed to query banned IPs: %v", err)
		return nil, err
	}
	defer rows.Close()

	var bannedIPs []BannedIP
	for rows.Next() {
		var bip BannedIP
		var expiresAt sql.NullTime
		err := rows.Scan(&bip.ID, &bip.IPAddress, &bip.BannedAt, &bip.Reason, &expiresAt)
		if err != nil {
			log.Errorf("Failed to scan banned IP: %v", err)
			return nil, err
		}
		if expiresAt.Valid {
			bip.ExpiresAt = &expiresAt.Time
		}
		bannedIPs = append(bannedIPs, bip)
	}
	log.Debugf("Retrieved %d banned IPs", len(bannedIPs))
	return bannedIPs, nil
}

// UpdateUserAvatar updates a user's avatar URL
func UpdateUserAvatar(username, avatarURL string) error {
	query := `
	UPDATE users
	SET avatar = ?
	WHERE username = ?
	`

	_, err := db.Exec(query, avatarURL, username)
	return err
}

// UpdateUserEmail updates a user's email address
func UpdateUserEmail(username, email string) error {
	query := `
	UPDATE users
	SET email = ?
	WHERE username = ?
	`

	_, err := db.Exec(query, email, username)
	return err
}
