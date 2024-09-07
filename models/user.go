package models

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2/log"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username            string `json:"username"`
	Password            string `json:"password"`
	RefreshTokenVersion int    `json:"refresh_token_version"`
	Role                string `json:"role"`
}

// CreateUser creates a new user with hashed password and default role.
func CreateUser(username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := User{
		Username:            username,
		Password:            string(hashedPassword),
		RefreshTokenVersion: 0,
		Role:                "reader", // Default role
	}

	count, err := CountUsers()
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if count == 0 {
		log.Infof("No users have yet been registered, promoting '%s' to 'admin' role", user.Username)
		user.Role = "admin"
	}

	return create("users", username, user)
}

// FindUserByUsername retrieves a user by their username.
func FindUserByUsername(username string) (*User, error) {
	var user User
	err := get("users", username, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by username: %w", err)
	}
	return &user, nil
}

// UpdateUserRole updates the role of a user.
func UpdateUserRole(username, newRole string) error {
	if !isValidRole(newRole) {
		return errors.New("invalid role")
	}

	user, err := FindUserByUsername(username)
	if err != nil {
		return err
	}

	user.Role = newRole
	return update("users", username, user)
}

// IncrementRefreshTokenVersion increments the refresh token version for a user.
func IncrementRefreshTokenVersion(username string) error {
	user, err := FindUserByUsername(username)
	if err != nil {
		return err
	}

	user.RefreshTokenVersion++
	return update("users", username, user)
}

// CountUsers returns the total number of users.
func CountUsers() (int64, error) {
	var dataList [][]byte
	if err := getAll("users", &dataList); err != nil {
		log.Errorf("Failed to get all users: %v", err)
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return int64(len(dataList)), nil
}

// isValidRole checks if the provided role is valid.
func isValidRole(role string) bool {
	switch role {
	case "reader", "moderator", "admin":
		return true
	default:
		return false
	}
}
