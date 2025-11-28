package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"encoding/json"
	"strings"
	"time"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// AccountListType represents the type of account list
type AccountListType string

const (
	AccountListFavorites AccountListType = "favorites"
	AccountListUpvoted   AccountListType = "upvoted"
	AccountListDownvoted AccountListType = "downvoted"
	AccountListReading   AccountListType = "reading"
)

// AccountListConfig holds configuration for account list types
type AccountListConfig struct {
	Title          string
	BreadcrumbLabel string
	EmptyMessage   string
	Path           string
}

// GetAccountListConfig returns the title and breadcrumb for an account list type
func GetAccountListConfig(listType string) (title string, breadcrumbLabel string, emptyMessage string, path string) {
	switch AccountListType(listType) {
	case AccountListFavorites:
		return "Favorites", "Favorites", "You have no favorites yet.", "/account/favorites"
	case AccountListUpvoted:
		return "Upvoted", "Upvoted", "You have not upvoted any media yet.", "/account/upvoted"
	case AccountListDownvoted:
		return "Downvoted", "Downvoted", "You have not downvoted any media yet.", "/account/downvoted"
	case AccountListReading:
		return "Reading", "Reading", "You are not reading any media right now.", "/account/reading"
	default:
		return "Unknown", "Unknown", "No items found.", ""
	}
}

// AccountListData holds data for account list pages
type AccountListData struct {
	Media       []models.Media
	TotalPages  int
	AllTags     []string
	SearchCount int
	Title       string
	EmptyMessage string
	Path        string
}

// GetAccountListData retrieves data for user account lists (favorites, upvoted, etc.)
func GetAccountListData(listType AccountListType, params models.QueryParams, userName string) (*AccountListData, error) {
	title, _, emptyMessage, path := GetAccountListConfig(string(listType))

	var getMediasFunc func(models.UserMediaListOptions) ([]models.Media, int, error)
	var getTagsFunc func(string) ([]string, error)

	switch listType {
	case AccountListFavorites:
		getMediasFunc = models.GetUserFavoritesWithOptions
		getTagsFunc = models.GetTagsForUserFavorites
	case AccountListUpvoted:
		getMediasFunc = models.GetUserUpvotedWithOptions
		getTagsFunc = models.GetTagsForUserUpvoted
	case AccountListDownvoted:
		getMediasFunc = models.GetUserDownvotedWithOptions
		getTagsFunc = models.GetTagsForUserDownvoted
	case AccountListReading:
		getMediasFunc = models.GetUserReadingWithOptions
		getTagsFunc = models.GetTagsForUserReading
	default:
		return nil, fmt.Errorf("unknown list type: %s", listType)
	}

	opts := models.UserMediaListOptions{
		Username:     userName,
		SearchFilter: params.SearchFilter,
		Page:         params.Page,
		PageSize:     16, // defaultPageSize
		SortBy:       params.Sort,
		SortOrder:    params.Order,
		Tags:         params.Tags,
		TagMode:      params.TagMode,
		Types:        params.Types,
	}

	media, count, err := getMediasFunc(opts)
	if err != nil {
		return nil, err
	}

	totalPages := CalculateTotalPages(int64(count), 16)

	// For account pages, we don't need premium countdowns, but populate anyway for consistency
	cfg, err := models.GetAppConfig()
	if err != nil {
		return nil, err
	}
	for i := range media {
		_, countdown, err := models.HasPremiumChapters(media[i].Slug, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled)
		if err != nil {
			log.Errorf("Error checking premium chapters for %s: %v", media[i].Slug, err)
		}
		media[i].PremiumCountdown = countdown
	}

	allTags, err := getTagsFunc(userName)
	if err != nil {
		return nil, err
	}

	return &AccountListData{
		Media:        media,
		TotalPages:   totalPages,
		AllTags:      allTags,
		SearchCount:  int(count),
		Title:        title,
		EmptyMessage: emptyMessage,
		Path:         path,
	}, nil
}

// UserMediaListData holds data for user media lists (favorites, reading, etc.)
type UserMediaListData struct {
	Media      []models.Media
	TotalCount int
	Tags       []string
}

// GetUserMediaListData gets media list data for a user
func GetUserMediaListData(userName, listType string, options models.UserMediaListOptions) (*UserMediaListData, error) {
	var getMediasFunc func(models.UserMediaListOptions) ([]models.Media, int, error)
	var getTagsFunc func(string) ([]string, error)

	switch listType {
	case "favorites":
		getMediasFunc = models.GetUserFavoritesWithOptions
		getTagsFunc = models.GetTagsForUserFavorites
	case "upvoted":
		getMediasFunc = models.GetUserUpvotedWithOptions
		getTagsFunc = models.GetTagsForUserUpvoted
	case "downvoted":
		getMediasFunc = models.GetUserDownvotedWithOptions
		getTagsFunc = models.GetTagsForUserDownvoted
	case "reading":
		getMediasFunc = models.GetUserReadingWithOptions
		getTagsFunc = models.GetTagsForUserReading
	default:
		return nil, nil // Unknown list type
	}

	media, count, err := getMediasFunc(options)
	if err != nil {
		return nil, err
	}

	tags, err := getTagsFunc(userName)
	if err != nil {
		return nil, err
	}

	return &UserMediaListData{
		Media:      media,
		TotalCount: count,
		Tags:       tags,
	}, nil
}

// HandleAccountList is the unified handler for all account media lists
func HandleAccountList(listType AccountListType) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userName := GetUserContext(c)
		if userName == "" {
			return fiber.ErrUnauthorized
		}

		params := ParseQueryParams(c)

		data, err := GetAccountListData(listType, params, userName)
		if err != nil {
			return handleError(c, err)
		}

		// HTMX fragment support
		if IsHTMXRequest(c) {
			target := GetHTMXTarget(c)
			if target == "account-listing" {
				return HandleView(c, views.AccountMediaListingWithTags(
					data.Media, params.Page, data.TotalPages, params.Sort, params.Order,
					data.Path, data.EmptyMessage, params.Tags, params.TagMode, data.AllTags, params.SearchFilter,
				))
			} else if target == "account-media-list-results" {
				return HandleView(c, views.MediaListingFragment(
					data.Media, params.Page, data.TotalPages, params.Sort, params.Order,
					data.EmptyMessage, data.Path, "account-media-list-results",
					params.Tags, params.TagMode, nil, params.SearchFilter,
				))
			}
		}

		// All views now call the standard AccountPageLayout function
		title, breadcrumbLabel, _, _ := GetAccountListConfig(string(listType))
		return HandleView(c, views.AccountPageLayout(
			title, breadcrumbLabel, data.Path, data.Media, params.Page, data.TotalPages, params.Sort, params.Order,
			data.EmptyMessage, data.AllTags, params.Tags, params.TagMode, params.SearchFilter,
		))
	}
}

// Account list route handlers
func HandleAccountFavorites(c *fiber.Ctx) error {
	return HandleAccountList(AccountListFavorites)(c)
}

func HandleAccountUpvoted(c *fiber.Ctx) error {
	return HandleAccountList(AccountListUpvoted)(c)
}

func HandleAccountDownvoted(c *fiber.Ctx) error {
	return HandleAccountList(AccountListDownvoted)(c)
}

func HandleAccountReading(c *fiber.Ctx) error {
	return HandleAccountList(AccountListReading)(c)
}

// HandleUsers renders the user administration view.
func HandleUsers(c *fiber.Ctx) error {
	return HandleView(c, views.Users())
}

// HandlePermissionsManagement renders the permissions management page
func HandlePermissionsManagement(c *fiber.Ctx) error {
	return HandleView(c, views.PermissionsManagement())
}

// HandleUserBan demotes and bans the specified user before returning the updated table.
func HandleUserBan(c *fiber.Ctx) error {
	username := c.Params("username")

	if err := models.BanUserWithDemotion(username); err != nil {
		return handleError(c, err)
	}

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

// HandleUserUnban lifts a user's ban and refreshes the table fragment.
func HandleUserUnban(c *fiber.Ctx) error {
	username := c.Params("username")

	models.UnbanUser(username)

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

// HandleUserPromote upgrades a user's role and returns the updated table.
func HandleUserPromote(c *fiber.Ctx) error {
	username := c.Params("username")

	if err := models.PromoteUser(username); err != nil {
		// For HTMX requests, return the table unchanged instead of an error
		// to avoid breaking the UI
		if IsHTMXRequest(c) {
			users, err := models.GetUsers()
			if err != nil {
				return handleError(c, err)
			}
			return HandleView(c, views.UsersTable(users))
		}
		return handleError(c, err)
	}

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

// HandleUserDemote reduces a user's role and refreshes the table view.
func HandleUserDemote(c *fiber.Ctx) error {
	username := c.Params("username")

	if err := models.DemoteUser(username); err != nil {
		// For HTMX requests, return the table unchanged instead of an error
		// to avoid breaking the UI
		if IsHTMXRequest(c) {
			users, err := models.GetUsers()
			if err != nil {
				return handleError(c, err)
			}
			return HandleView(c, views.UsersTable(users))
		}
		return handleError(c, err)
	}

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

// HandleAccount renders the current user's account page showing favorites, reading lists and liked media
func HandleAccount(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	return HandleView(c, views.Account(userName))
}

// HandleBannedIPs displays the list of banned IPs
func HandleBannedIPs(c *fiber.Ctx) error {
	return HandleView(c, views.BannedIPs())
}

// HandleUnbanIP removes an IP from the banned list
func HandleUnbanIP(c *fiber.Ctx) error {
	ip := c.Params("ip")
	if ip == "" {
		return c.Status(fiber.StatusBadRequest).SendString("IP address is required")
	}

	err := models.UnbanIP(ip)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to unban IP")
	}

	bannedIPs, err := models.GetBannedIPs()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.BannedIPsTable(bannedIPs))
}

// HandleExternalAccounts shows the external accounts page
func HandleExternalAccounts(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// Get user's external accounts
	accounts, err := models.GetUserExternalAccounts(userName)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ExternalAccountsPage(accounts))
}

// HandleConnectMAL saves MAL credentials
func HandleConnectMAL(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	clientID := c.FormValue("client_id")
	clientSecret := c.FormValue("client_secret")
	if clientID == "" || clientSecret == "" {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Client ID and Secret required"))
	}

	// Save the account with client_id and client_secret
	account := &models.UserExternalAccount{
		UserName:    userName,
		ServiceName: "mal",
		AccessToken: clientID,    // Store client_id here
		RefreshToken: clientSecret, // Store client_secret here
	}
	err := models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect("/account/external")
}

// HandleAuthorizeMAL redirects to MAL for OAuth authorization
func HandleAuthorizeMAL(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// Get the stored credentials
	account, err := models.GetUserExternalAccount(userName, "mal")
	if err != nil {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "No MAL credentials found"))
	}

	clientID := account.AccessToken
	if clientID == "" {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Client ID not set"))
	}

	// Generate PKCE code verifier and challenge
	codeVerifier := generateCodeVerifier()
	codeChallenge := codeVerifier // For plain method

	// Generate state for security
	state := userName + "|" + generateCodeVerifier()[:8]

	redirectURI := "http://localhost:3000/callback" // Must match MAL app config

	authURL := fmt.Sprintf("https://myanimelist.net/v1/oauth2/authorize?response_type=code&client_id=%s&redirect_uri=%s&code_challenge=%s&code_challenge_method=plain&state=%s",
		clientID, redirectURI, codeChallenge, state)

	// Store code_verifier and state temporarily
	account.ExternalUserID = codeVerifier + "|" + state
	err = models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect(authURL)
}

// HandleDisconnectMAL disconnects the MyAnimeList account
func HandleDisconnectMAL(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	err := models.DeleteUserExternalAccount(userName, "mal")
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect("/account/external")
}

// generateCodeVerifier generates a random code verifier for PKCE
func generateCodeVerifier() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_~"
	b := make([]byte, 43)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// generateCodeChallenge generates the code challenge from verifier using S256
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// HandleMALCallback handles the OAuth callback from MAL
func HandleMALCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "No code or state provided"))
	}

	// Parse state to get userName
	parts := strings.Split(state, "|")
	if len(parts) != 2 {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Invalid state"))
	}
	userName := parts[0]

	account, err := models.GetUserExternalAccount(userName, "mal")
	if err != nil {
		return handleError(c, err)
	}

	storedParts := strings.Split(account.ExternalUserID, "|")
	if len(storedParts) != 3 {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Invalid stored data"))
	}
	storedVerifier := storedParts[0]
	storedState := storedParts[1] + "|" + storedParts[2]
	if storedState != state {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "State mismatch"))
	}
	codeVerifier := storedVerifier

	clientID := account.AccessToken
	clientSecret := account.RefreshToken

	// Exchange code for token
	tokenURL := "https://myanimelist.net/v1/oauth2/token"
	data := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=authorization_code&code=%s&redirect_uri=%s&code_verifier=%s",
		url.QueryEscape(clientID), url.QueryEscape(clientSecret), code, url.QueryEscape("http://localhost:3000/callback"), codeVerifier)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data))
	if err != nil {
		return handleError(c, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return handleError(c, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error(fmt.Sprintf("Token exchange failed: %d, body: %s", resp.StatusCode, string(body)))
		return handleError(c, fmt.Errorf("Token exchange failed: %d", resp.StatusCode))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return handleError(c, err)
	}

	// Update the account with the access token
	account.AccessToken = tokenResp.AccessToken
	account.RefreshToken = tokenResp.RefreshToken
	account.TokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	account.ExternalUserID = "" // Clear
	err = models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect("/account/external")
}