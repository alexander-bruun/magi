package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
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
	Username string `json:"username"`
	Password string `json:"password"`
}

// GetAuthConfig gets authentication configuration
func GetAuthConfig() (*AuthConfig, error) {
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
func CreateUser(username, password string) error {
	return models.CreateUser(username, password)
}

// CanRegister checks if registration is allowed
func CanRegister() (bool, error) {
	config, err := GetAuthConfig()
	if err != nil {
		return false, err
	}

	return config.AllowRegistration && (config.MaxUsers == 0 || config.UserCount < config.MaxUsers), nil
}

// RegisterHandler renders the registration form page.
func RegisterHandler(c *fiber.Ctx) error {
	canRegister, err := CanRegister()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	if !canRegister {
		if IsHTMXRequest(c) {
			return sendForbiddenError(c, "Registration is currently disabled.")
		}
		return HandleView(c, views.Error("Registration is currently disabled."))
	}

	return HandleView(c, views.Register())
}

// LoginHandler renders the login page.
func LoginHandler(c *fiber.Ctx) error {
	target := c.Query("target", "")
	return HandleView(c, views.Login(target))
}

// CreateUserHandler processes a registration submission and redirects to login on success.
func CreateUserHandler(c *fiber.Ctx) error {
	canRegister, err := CanRegister()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	if !canRegister {
		return sendForbiddenError(c, "Registration is currently disabled.")
	}

	var formData RegisterFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	username := formData.Username
	password := formData.Password

	if err := CreateUser(username, password); err != nil {
		// Provide specific error messages based on the error
		var errorMsg string
		if err.Error() == "username already exists" {
			errorMsg = ErrUsernameExists
		} else if err.Error() == "username too short" {
			errorMsg = ErrUsernameTooShort
		} else if err.Error() == "username too long" {
			errorMsg = ErrUsernameTooLong
		} else if err.Error() == "password too weak" {
			errorMsg = ErrPasswordTooWeak
		} else {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		
		// For HTMX requests, return validation error with notification
		if IsHTMXRequest(c) {
			return sendValidationError(c, errorMsg)
		}
		// For regular requests, show error page
		return HandleView(c, views.Error(errorMsg))
	}

	// Automatically log in the user after registration
	sessionToken, err := models.CreateSessionToken(username)
	if err != nil {
		return sendInternalServerError(c, "Could not create session after registration", err)
	}

	setSessionCookie(c, sessionToken)
	
	// Add success notification for HTMX requests
	triggerNotification(c, "Account created successfully! Welcome to Magi.", "success")
	
	c.Set("HX-Redirect", "/")
	return c.SendStatus(fiber.StatusCreated)
}

// LoginUserHandler validates credentials and issues a session token.
func LoginUserHandler(c *fiber.Ctx) error {
	var formData LoginFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	username := formData.Username
	password := formData.Password

	user, err := models.FindUserByUsername(username)
	if err != nil || user == nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		// For HTMX requests, return unauthorized error with notification
		if IsHTMXRequest(c) {
			return sendUnauthorizedError(c, ErrLoginFailed)
		}
		// For regular requests, show wrong credentials view
		return HandleView(c, views.WrongCredentials())
	}

	sessionToken, err := models.CreateSessionToken(user.Username)
	if err != nil {
		return sendInternalServerError(c, "Could not create session after login", err)
	}

	setSessionCookie(c, sessionToken)
	
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

// LogoutHandler revokes the session token and clears authentication cookie.
func LogoutHandler(c *fiber.Ctx) error {
	sessionToken := c.Cookies("session_token")

	if sessionToken != "" {
		models.DeleteSessionToken(sessionToken)
	}

	clearSessionCookie(c)
	c.Set("HX-Redirect", "/")
	return c.SendStatus(fiber.StatusOK)
}
