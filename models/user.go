package models

import (
	"errors"

	"github.com/gofiber/fiber/v2/log"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username            string `json:"username"`
	Password            string `json:"password"`
	RefreshTokenVersion int    `json:"refresh_token_version"`
	Role                string `json:"role"`
}

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
		log.Infof("No users have yet been registered, promoting '%s' to 'admin' role.", user.Username)
		user.Role = "admin"
	}

	return create("users", username, user)
}

func FindUserByUsername(username string) (*User, error) {
	var user User
	err := get("users", username, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func UpdateUserRole(username string, newRole string) error {
	if newRole != "reader" && newRole != "moderator" && newRole != "admin" {
		return errors.New("invalid role")
	}

	user, err := FindUserByUsername(username)
	if err != nil {
		return err
	}

	user.Role = newRole
	return update("users", username, user)
}

func IncrementRefreshTokenVersion(user *User) error {
	if user == nil {
		return errors.New("user is nil")
	}

	user.RefreshTokenVersion++
	return update("users", user.Username, user)
}

func CountUsers() (int64, error) {
	dataList, err := getAll("users")
	if err != nil {
		return 0, err
	}
	return int64(len(dataList)), nil
}
