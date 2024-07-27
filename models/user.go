package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	gorm.Model
	Username            string `gorm:"uniqueIndex"`
	Password            string
	RefreshTokenVersion uint
	Role                string
}

// CreateUser creates a new user in the database
func (u *User) CreateUser() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return db.Create(u).Error
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
func (u *User) UpdateUserRole(newRole string) error {
	if newRole != "reader" && newRole != "moderator" && newRole != "admin" {
		return gorm.ErrInvalidData // Customize this error as needed
	}
	u.Role = newRole
	return db.Save(u).Error
}

// IncrementRefreshTokenVersion increments the refresh token version
func (u *User) IncrementRefreshTokenVersion() error {
	u.RefreshTokenVersion++
	return db.Save(u).Error
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
