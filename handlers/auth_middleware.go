package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/models"
	emailutil "github.com/alexander-bruun/magi/utils/email"
	"github.com/alexander-bruun/magi/utils/files"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"golang.org/x/crypto/bcrypt"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	AllowRegistration bool
	MaxUsers          int
	UserCount         int
}

// LoginFormData represents login form data
type LoginFormData struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Target   string `json:"target"`
}

// RegisterFormData represents registration form data
type RegisterFormData struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	Email           string `json:"email"`
}

// getAuthConfig gets authentication configuration
func getAuthConfig() (*AuthConfig, error) {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return nil, err
	}

	count, err := models.CountUsers()
	if err != nil {
		return nil, err
	}

	return &AuthConfig{
		AllowRegistration: cfg.AllowRegistration,
		MaxUsers:          int(cfg.MaxUsers),
		UserCount:         int(count),
	}, nil
}

// CreateUser creates a new user
func createUser(username, password, email string) error {
	return models.CreateUser(username, password, email)
}

// canRegister checks if registration is allowed
func canRegister() (bool, error) {
	config, err := getAuthConfig()
	if err != nil {
		return false, err
	}

	return config.AllowRegistration && (config.MaxUsers == 0 || config.UserCount < config.MaxUsers), nil
}

// registerHandler renders the registration form page.
func registerHandler(c fiber.Ctx) error {
	canRegister, err := canRegister()
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	if !canRegister {
		if isHTMXRequest(c) {
			return SendForbiddenError(c, "Registration is currently disabled.")
		}
		return handleView(c, views.Error("Registration is currently disabled."))
	}

	return handleView(c, views.Register())
}

// loginHandler renders the login page.
func loginHandler(c fiber.Ctx) error {
	target := c.Query("target", "")
	return handleView(c, views.Login(target))
}

// createUserHandler processes a registration submission and redirects to login on success.
func createUserHandler(c fiber.Ctx) error {
	canRegister, err := canRegister()
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	if !canRegister {
		return SendForbiddenError(c, "Registration is currently disabled.")
	}

	// Parse multipart form fields
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")
	confirmPassword := c.FormValue("confirm_password")
	email := strings.TrimSpace(c.FormValue("email"))

	if username == "" || password == "" || email == "" {
		return SendBadRequestError(c, ErrBadRequest)
	}

	if password != confirmPassword {
		if isHTMXRequest(c) {
			return SendValidationError(c, ErrPasswordMismatch)
		}
		return handleView(c, views.Error(ErrPasswordMismatch))
	}

	// Block disposable email providers
	if emailutil.IsDisposableEmail(email) {
		if isHTMXRequest(c) {
			return SendValidationError(c, ErrDisposableEmail)
		}
		return handleView(c, views.Error(ErrDisposableEmail))
	}

	if err := createUser(username, password, email); err != nil {
		// Provide specific error messages based on the error
		var errorMsg string
		if err.Error() == "username already exists" {
			errorMsg = ErrUsernameExists
		} else if err.Error() == "email already exists" {
			errorMsg = ErrEmailExists
		} else if err.Error() == "username too short" {
			errorMsg = ErrUsernameTooShort
		} else if err.Error() == "username too long" {
			errorMsg = ErrUsernameTooLong
		} else if err.Error() == "password too weak" {
			errorMsg = ErrPasswordTooWeak
		} else {
			return SendInternalServerError(c, ErrInternalServerError, err)
		}

		// For HTMX requests, return validation error with notification
		if isHTMXRequest(c) {
			return SendValidationError(c, errorMsg)
		}
		// For regular requests, show error page
		return handleView(c, views.Error(errorMsg))
	}

	// Handle optional avatar upload
	if file, err := c.FormFile("avatar"); err == nil && file != nil {
		if file.Size <= 2*1024*1024 {
			contentType := file.Header.Get("Content-Type")
			if contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/gif" {
				ext := ".jpg"
				switch contentType {
				case "image/png":
					ext = ".png"
				case "image/gif":
					ext = ".gif"
				}
				filename := fmt.Sprintf("%s_%d%s", username, time.Now().Unix(), ext)
				avatarsDir := filepath.Join(files.GetDataDirectory(), "avatars")
				if err := os.MkdirAll(avatarsDir, 0755); err == nil {
					filePath := filepath.Join(avatarsDir, filename)
					if err := c.SaveFile(file, filePath); err == nil {
						avatarURL := fmt.Sprintf("/api/avatars/%s", filename)
						if err := models.UpdateUserAvatar(username, avatarURL); err != nil {
							os.Remove(filePath)
							log.Warnf("Failed to save avatar for new user '%s': %v", username, err)
						}
					}
				}
			}
		}
	}

	// Automatically log in the user after registration
	sessionToken, err := models.CreateSessionToken(username)
	if err != nil {
		return SendInternalServerError(c, "Could not create session after registration", err)
	}

	SetSessionCookie(c, sessionToken)

	// Add success notification for HTMX requests
	triggerNotification(c, "Account created successfully! Welcome to Magi.", "success")

	c.Set("HX-Redirect", "/")
	return c.SendStatus(fiber.StatusCreated)
}

// loginUserHandler validates credentials and issues a session token.
func loginUserHandler(c fiber.Ctx) error {
	var formData LoginFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	username := formData.Username
	password := formData.Password

	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		RecordFailedLogin(getRealIP(c))
		// For HTMX requests, return unauthorized error with notification
		if isHTMXRequest(c) {
			return SendUnauthorizedError(c, ErrLoginFailed)
		}
		// For regular requests, show wrong credentials view
		return handleView(c, views.WrongCredentials())
	}

	ClearLoginAttempts(getRealIP(c))

	sessionToken, err := models.CreateSessionToken(user.Username)
	if err != nil {
		return SendInternalServerError(c, "Could not create session after login", err)
	}

	SetSessionCookie(c, sessionToken)

	// Add success notification for HTMX requests
	triggerNotification(c, "Login successful! Welcome back.", "success")

	// Redirect to target page if provided, otherwise home
	target := formData.Target
	if target == "" {
		target = "/"
	}
	c.Set("HX-Redirect", target)
	return c.SendStatus(fiber.StatusOK)
}

// logoutHandler revokes the session token and clears authentication cookie.
func logoutHandler(c fiber.Ctx) error {
	sessionToken := c.Cookies("session_token")

	if sessionToken != "" {
		models.DeleteSessionToken(sessionToken)
	}

	ClearSessionCookie(c)
	c.Set("HX-Redirect", "/")
	return c.SendStatus(fiber.StatusOK)
}
