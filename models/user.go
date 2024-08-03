package models

import (
	"github.com/gofiber/fiber/v2/log"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	gorm.Model
	Username            string `gorm:"unique;not null"`
	Password            string `gorm:"not null"`
	RefreshTokenVersion int
	Role                string
}

// CreateUser creates a new user in the database
func CreateUser(username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := User{
		Username:            username,
		Password:            string(hashedPassword),
		RefreshTokenVersion: 0,
		Role:                "reader", // Default role
	}

	count, _ := CountUsers()
	if count == 0 {
		log.Infof("No users has yet been registered, promoting '%s' to 'admin' role.", user.Username)
		user.Role = "admin"
	}

	err = db.Create(&user).Error
	if err != nil {
		log.Errorf("Error creating user: %v", err)
		return err
	}

	return nil
}

// FindUserByUsername retrieves a user by username
func FindUserByUsername(username string) (*User, error) {
	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUserRole updates the user's role in the database
func UpdateUserRole(userID uint, newRole string) error {
	if newRole != "reader" && newRole != "moderator" && newRole != "admin" {
		return gorm.ErrInvalidData // Customize this error as needed
	}
	return db.Model(&User{}).Where("id = ?", userID).Update("role", newRole).Error
}

// IncrementRefreshTokenVersion increments the refresh token version for a given user and returns the updated user
func IncrementRefreshTokenVersion(user *User) error {
	// Check if the user object is not nil
	if user == nil {
		return gorm.ErrRecordNotFound // or a custom error indicating the user is nil
	}

	// Increment the refresh token version
	if err := db.Model(user).UpdateColumn("refresh_token_version", gorm.Expr("refresh_token_version + ?", 1)).Error; err != nil {
		return err
	}

	// Fetch the updated user to ensure we have the latest version
	if err := db.Where("id = ?", user.ID).First(user).Error; err != nil {
		return err
	}

	return nil
}

// CountUsers retrieves the total number of users in the database
func CountUsers() (int64, error) {
	var count int64
	err := db.Model(&User{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}
