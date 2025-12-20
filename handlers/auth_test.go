package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestGetAuthConfig(t *testing.T) {
	// Test default config
	config, err := GetAuthConfig()
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, true, config.AllowRegistration) // Default should be true
	assert.Equal(t, 0, int(config.MaxUsers))      // Default should be 0
	assert.Equal(t, 0, config.UserCount)            // No users initially
}

func TestCanRegister(t *testing.T) {
	// Test with default config (registration enabled)
	canRegister, err := CanRegister()
	assert.NoError(t, err)
	assert.True(t, canRegister)

	// Disable registration
	_, err = models.UpdateAppConfig(false, 10, 0)
	assert.NoError(t, err)

	canRegister, err = CanRegister()
	assert.NoError(t, err)
	assert.False(t, canRegister)

	// Re-enable and test with user limit
	_, err = models.UpdateAppConfig(true, 10, 0)
	assert.NoError(t, err)

	canRegister, err = CanRegister()
	assert.NoError(t, err)
	assert.True(t, canRegister)

	// Test with user limit reached
	// Create 10 users to reach the limit
	for i := 0; i < 10; i++ {
		err = models.CreateUser("testuser"+string(rune(i+48)), "password123")
		assert.NoError(t, err)
	}

	canRegister, err = CanRegister()
	assert.NoError(t, err)
	assert.False(t, canRegister) // Should be false when at max users
}

func TestCreateUser(t *testing.T) {
	// Test successful user creation
	err := CreateUser("auth_testuser", "password123")
	assert.NoError(t, err)

	// Verify user was created
	user, err := models.FindUserByUsername("auth_testuser")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "auth_testuser", user.Username)

	// Test duplicate username (this will fail at database level)
	err = CreateUser("auth_testuser", "differentpassword")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")

	// Test creating another valid user
	err = CreateUser("auth_testuser2", "password123")
	assert.NoError(t, err)
}

func TestRegisterHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/register", RegisterHandler)

	// Reset to default config (registration enabled)
	_, err := models.UpdateAppConfig(true, 0, 3)
	assert.NoError(t, err)

	// Test with registration enabled (default)
	req := httptest.NewRequest("GET", "/register", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode) // OK

	// Disable registration
	_, err = models.UpdateAppConfig(false, 10, 0)
	assert.NoError(t, err)

	// Test with registration disabled
	req = httptest.NewRequest("GET", "/register", nil)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode) // OK (returns error page)
}

func TestLoginHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/login", LoginHandler)

	// Test basic login page render
	req := httptest.NewRequest("GET", "/login", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Test with target parameter
	req = httptest.NewRequest("GET", "/login?target=/dashboard", nil)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestCreateUserHandler(t *testing.T) {
	app := fiber.New()
	app.Post("/register", CreateUserHandler)

	// Reset to default config (registration enabled)
	_, err := models.UpdateAppConfig(true, 0, 3)
	assert.NoError(t, err)

	// Test successful registration
	formData := "username=newuser&password=password123"
	req := httptest.NewRequest("POST", "/register", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode) // Created

	// Verify user was created
	user, err := models.FindUserByUsername("newuser")
	assert.NoError(t, err)
	assert.NotNil(t, user)

	// Test duplicate username
	formData = "username=newuser&password=differentpass"
	req = httptest.NewRequest("POST", "/register", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode) // Internal Server Error (database constraint violation)

	// Test creating another valid user
	formData = "username=anotheruser&password=password123"
	req = httptest.NewRequest("POST", "/register", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode) // Created
}

func TestLoginUserHandler(t *testing.T) {
	app := fiber.New()
	app.Post("/login", LoginUserHandler)

	// Create a test user first
	err := models.CreateUser("loginuser", "password123")
	assert.NoError(t, err)

	// Test successful login
	formData := "username=loginuser&password=password123"
	req := httptest.NewRequest("POST", "/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode) // OK

	// Check for session cookie
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "session_token" {
			sessionCookie = cookie
			break
		}
	}
	assert.NotNil(t, sessionCookie)
	assert.NotEmpty(t, sessionCookie.Value)

	// Test invalid credentials
	formData = "username=loginuser&password=wrongpassword"
	req = httptest.NewRequest("POST", "/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true") // Add HTMX header
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode) // Unauthorized

	// Test non-existent user
	formData = "username=nonexistent&password=password123"
	req = httptest.NewRequest("POST", "/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true") // Add HTMX header
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode) // Unauthorized
}

func TestLogoutHandler(t *testing.T) {
	app := fiber.New()
	app.Post("/logout", LogoutHandler)

	// Create a test user and session
	err := models.CreateUser("logout_testuser", "password123")
	assert.NoError(t, err)

	sessionToken, err := models.CreateSessionToken("logout_testuser")
	assert.NoError(t, err)

	// Test logout with valid session
	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sessionToken})
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode) // OK

	// Check that session cookie is cleared
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "session_token" {
			sessionCookie = cookie
			break
		}
	}
	assert.NotNil(t, sessionCookie)
	assert.Equal(t, "", sessionCookie.Value) // Should be cleared

	// Test logout without session (should not error)
	req = httptest.NewRequest("POST", "/logout", nil)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode) // OK
}