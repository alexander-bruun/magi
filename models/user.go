package models

import (
	"encoding/json"
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
	Banned              bool   `json:"banned"`
}

// GetUsers retrieves all Users from the database
func GetUsers() ([]User, error) {
	var dataList [][]byte
	if err := getAll("users", &dataList); err != nil {
		log.Errorf("Failed to get all users: %v", err)
		return nil, err
	}

	var users []User
	for _, data := range dataList {
		var user User
		if err := json.Unmarshal(data, &user); err != nil {
			log.Errorf("Failed to unmarshal user data: %v", err)
			continue
		}
		users = append(users, user)
	}
	return users, nil
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

// BanUser bans a user by setting the Banned field to true.
func BanUser(username string) error {
	user, err := FindUserByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to find user to ban: %w", err)
	}

	if user.Banned {
		return fmt.Errorf("user '%s' is already banned", username)
	}

	user.Banned = true
	if err := update("users", username, user); err != nil {
		return fmt.Errorf("failed to ban user: %w", err)
	}

	log.Infof("User '%s' has been banned", username)
	return nil
}

// UnbanUser unbans a user by setting the Banned field to false.
func UnbanUser(username string) error {
	user, err := FindUserByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to find user to unban: %w", err)
	}

	if !user.Banned {
		return fmt.Errorf("user '%s' is not banned", username)
	}

	user.Banned = false
	if err := update("users", username, user); err != nil {
		return fmt.Errorf("failed to unban user: %w", err)
	}

	log.Infof("User '%s' has been unbanned", username)
	return nil
}
