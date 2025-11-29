package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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
	// Load initial data for the table
	users, total, err := models.GetUsersWithOptions(models.UserSearchOptions{
		Filter:   "",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersWithData(users, 1, 10, total, ""))
}

// HandleUsersTable renders the users table fragment with pagination and search.
func HandleUsersTable(c *fiber.Ctx) error {
	page := 1
	pageSize := 10
	filter := ""

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	filter = c.Query("filter")

	users, total, err := models.GetUsersWithOptions(models.UserSearchOptions{
		Filter:   filter,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users, page, pageSize, total, filter))
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

	page := 1
	pageSize := 10
	filter := ""

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	filter = c.Query("filter")

	users, total, err := models.GetUsersWithOptions(models.UserSearchOptions{
		Filter:   filter,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users, page, pageSize, total, filter))
}

// HandleUserUnban lifts a user's ban and refreshes the table fragment.
func HandleUserUnban(c *fiber.Ctx) error {
	username := c.Params("username")

	models.UnbanUser(username)

	page := 1
	pageSize := 10
	filter := ""

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	filter = c.Query("filter")

	users, total, err := models.GetUsersWithOptions(models.UserSearchOptions{
		Filter:   filter,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users, page, pageSize, total, filter))
}

// HandleUserPromote upgrades a user's role and returns the updated table.
func HandleUserPromote(c *fiber.Ctx) error {
	username := c.Params("username")

	if err := models.PromoteUser(username); err != nil {
		// For HTMX requests, return the table unchanged instead of an error
		// to avoid breaking the UI
		if IsHTMXRequest(c) {
			page := 1
			pageSize := 10
			filter := ""

			if p := c.Query("page"); p != "" {
				if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
					page = parsed
				}
			}
			if ps := c.Query("pageSize"); ps != "" {
				if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
					pageSize = parsed
				}
			}
			filter = c.Query("filter")

			users, total, err := models.GetUsersWithOptions(models.UserSearchOptions{
				Filter:   filter,
				Page:     page,
				PageSize: pageSize,
			})
			if err != nil {
				return handleError(c, err)
			}
			return HandleView(c, views.UsersTable(users, page, pageSize, total, filter))
		}
		return handleError(c, err)
	}

	page := 1
	pageSize := 10
	filter := ""

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	filter = c.Query("filter")

	users, total, err := models.GetUsersWithOptions(models.UserSearchOptions{
		Filter:   filter,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users, page, pageSize, total, filter))
}

// HandleUserDemote reduces a user's role and refreshes the table view.
func HandleUserDemote(c *fiber.Ctx) error {
	username := c.Params("username")

	if err := models.DemoteUser(username); err != nil {
		// For HTMX requests, return the table unchanged instead of an error
		// to avoid breaking the UI
		if IsHTMXRequest(c) {
			page := 1
			pageSize := 10
			filter := ""

			if p := c.Query("page"); p != "" {
				if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
					page = parsed
				}
			}
			if ps := c.Query("pageSize"); ps != "" {
				if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
					pageSize = parsed
				}
			}
			filter = c.Query("filter")

			users, total, err := models.GetUsersWithOptions(models.UserSearchOptions{
				Filter:   filter,
				Page:     page,
				PageSize: pageSize,
			})
			if err != nil {
				return handleError(c, err)
			}
			return HandleView(c, views.UsersTable(users, page, pageSize, total, filter))
		}
		return handleError(c, err)
	}

	page := 1
	pageSize := 10
	filter := ""

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	filter = c.Query("filter")

	users, total, err := models.GetUsersWithOptions(models.UserSearchOptions{
		Filter:   filter,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users, page, pageSize, total, filter))
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
		UserName:     userName,
		ServiceName:  "mal",
		ExternalUserID: clientID, // Store client_id here
		RefreshToken: clientSecret, // Store client_secret here
	}
	err := models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	// Fetch updated accounts and return the view
	accounts, err := models.GetUserExternalAccounts(userName)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ExternalAccountsPage(accounts))
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

	clientID := account.ExternalUserID
	if clientID == "" {
		// Fallback for old accounts where client_id was stored in AccessToken
		if account.AccessToken != "" && len(account.AccessToken) < 30 {
			clientID = account.AccessToken
		}
	}
	if clientID == "" {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Client ID not set"))
	}

	// Generate PKCE code verifier and challenge
	codeVerifier := generateCodeVerifier()
	codeChallenge := codeVerifier // For plain method

	// Generate state for security
	state := userName + "|" + codeVerifier[:8]

	redirectURI := "http://localhost:3000/external/callback/mal" // Must match MAL app config

	authURL := fmt.Sprintf("https://myanimelist.net/v1/oauth2/authorize?response_type=code&client_id=%s&redirect_uri=%s&code_challenge=%s&code_challenge_method=plain&state=%s",
		clientID, redirectURI, codeChallenge, state)

	// Store code_verifier and state temporarily
	account.AccessToken = codeVerifier + "|" + state
	err = models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect(authURL)
}

// HandleConnectAniList saves AniList credentials
func HandleConnectAniList(c *fiber.Ctx) error {
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
		UserName:     userName,
		ServiceName:  "anilist",
		ExternalUserID: clientID, // Store client_id here
		RefreshToken: clientSecret, // Store client_secret here
	}
	err := models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	// Fetch updated accounts and return the view
	accounts, err := models.GetUserExternalAccounts(userName)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ExternalAccountsPage(accounts))
}

// HandleAuthorizeAniList redirects to AniList for OAuth authorization
func HandleAuthorizeAniList(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// Get the stored credentials
	account, err := models.GetUserExternalAccount(userName, "anilist")
	if err != nil {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "No AniList credentials found"))
	}

	clientID := account.ExternalUserID
	if clientID == "" {
		// Fallback for old accounts where client_id was stored in AccessToken
		if account.AccessToken != "" && len(account.AccessToken) < 30 {
			clientID = account.AccessToken
		}
	}
	if clientID == "" {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Client ID not set"))
	}

	// Generate state for security
	state := userName + "|" + generateCodeVerifier()[:8]

	redirectURI := "http://localhost:3000/external/callback/anilist" // Must match AniList app config

	authURL := fmt.Sprintf("https://anilist.co/api/v2/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&state=%s",
		clientID, redirectURI, state)

	// Store state temporarily
	account.AccessToken = state
	err = models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect(authURL)
}

// HandleDisconnectAniList disconnects the AniList account
func HandleDisconnectAniList(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	err := models.DeleteUserExternalAccount(userName, "anilist")
	if err != nil {
		return handleError(c, err)
	}

	// Fetch updated accounts and return the view
	accounts, err := models.GetUserExternalAccounts(userName)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ExternalAccountsPage(accounts))
}


// HandleDisconnectMAL disconnects the MAL account
func HandleDisconnectMAL(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	err := models.DeleteUserExternalAccount(userName, "mal")
	if err != nil {
		return handleError(c, err)
	}

	// Fetch updated accounts and return the view
	accounts, err := models.GetUserExternalAccounts(userName)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ExternalAccountsPage(accounts))
}

// HandleMALCallback handles OAuth callback from MAL
func HandleMALCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Missing code or state"))
	}

	// Parse state: userName|suffix
	parts := strings.Split(state, "|")
	if len(parts) != 2 {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Invalid state"))
	}
	userName := parts[0]

	// Get MAL account
	account, err := models.GetUserExternalAccount(userName, "mal")
	if err != nil {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "No MAL account found"))
	}

	storedParts := strings.Split(account.AccessToken, "|")
	if len(storedParts) < 2 {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Invalid stored state"))
	}
	reconstructedState := strings.Join(storedParts[len(storedParts)-2:], "|")
	if reconstructedState != state {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "State mismatch"))
	}
	codeVerifier := storedParts[0]

	return exchangeMALToken(c, account, code, codeVerifier)
}

// HandleAniListCallback handles OAuth callback from AniList
func HandleAniListCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Missing code or state"))
	}

	// Parse state: userName|suffix
	parts := strings.Split(state, "|")
	if len(parts) != 2 {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Invalid state"))
	}
	userName := parts[0]

	// Get AniList account
	account, err := models.GetUserExternalAccount(userName, "anilist")
	if err != nil {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "No AniList account found"))
	}

	storedParts := strings.Split(account.AccessToken, "|")
	if len(storedParts) < 2 {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "Invalid stored state"))
	}
	reconstructedState := strings.Join(storedParts[len(storedParts)-2:], "|")
	if reconstructedState != state {
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, "State mismatch"))
	}

	return exchangeAniListToken(c, account, code)
}

// exchangeMALToken exchanges authorization code for MAL access token
func exchangeMALToken(c *fiber.Ctx, account *models.UserExternalAccount, code, codeVerifier string) error {
	clientID := account.ExternalUserID
	clientSecret := account.RefreshToken

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("code_verifier", codeVerifier)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", "http://localhost:3000/external/callback/mal")

	resp, err := http.PostForm("https://myanimelist.net/v1/oauth2/token", data)
	if err != nil {
		return handleError(c, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Failed to exchange token: %s", string(body))))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return handleError(c, err)
	}

	// Update account with tokens
	account.AccessToken = tokenResp.AccessToken
	account.RefreshToken = tokenResp.RefreshToken
	// Note: MAL doesn't provide expires_in in response, but typically 30 days
	// For now, set to 0 or handle later

	err = models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	// Fetch updated accounts and return the view
	accounts, err := models.GetUserExternalAccounts(account.UserName)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ExternalAccountsPage(accounts))
}

// exchangeAniListToken exchanges authorization code for AniList access token
func exchangeAniListToken(c *fiber.Ctx, account *models.UserExternalAccount, code string) error {
	clientID := account.ExternalUserID
	clientSecret := account.RefreshToken

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", "http://localhost:3000/external/callback/anilist")
	data.Set("code", code)

	resp, err := http.PostForm("https://anilist.co/api/v2/oauth/token", data)
	if err != nil {
		return handleError(c, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return handleError(c, fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Failed to exchange token: %s", string(body))))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return handleError(c, err)
	}

	// Update account with token
	account.AccessToken = tokenResp.AccessToken
	// AniList doesn't provide refresh token

	err = models.SaveUserExternalAccount(account)
	if err != nil {
		return handleError(c, err)
	}

	// Fetch updated accounts and return the view
	accounts, err := models.GetUserExternalAccounts(account.UserName)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.ExternalAccountsPage(accounts))
}

// HandleUploadAvatar handles avatar image uploads
func HandleUploadAvatar(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// Get current user to check for existing avatar
	currentUser, err := models.FindUserByUsername(userName)
	if err != nil {
		return handleError(c, err)
	}
	if currentUser == nil {
		return fiber.ErrUnauthorized
	}

	// Delete old avatar file if exists
	if currentUser.Avatar != "" {
		oldFilename := strings.TrimPrefix(currentUser.Avatar, "/api/avatars/")
		oldFilepath := fmt.Sprintf("./cache/avatars/%s", oldFilename)
		if err := os.Remove(oldFilepath); err != nil && !os.IsNotExist(err) {
			// Log but don't fail the request
			log.Warnf("Failed to delete old avatar file %s: %v", oldFilepath, err)
		}
	}

	// Get the uploaded file
	file, err := c.FormFile("avatar")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("No file uploaded")
	}

	// Validate file size (max 2MB)
	if file.Size > 2*1024*1024 {
		return c.Status(fiber.StatusBadRequest).SendString("File too large. Maximum size is 2MB")
	}

	// Validate file type
	contentType := file.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid file type. Only JPG, PNG, and GIF are allowed")
	}

	// Generate unique filename
	ext := ".jpg"
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	}
	filename := fmt.Sprintf("%s_%d%s", userName, time.Now().Unix(), ext)
	filepath := fmt.Sprintf("./cache/avatars/%s", filename)

	// Ensure avatars directory exists
	if err := os.MkdirAll("./cache/avatars", 0755); err != nil {
		return handleError(c, err)
	}

	// Save the file
	if err := c.SaveFile(file, filepath); err != nil {
		return handleError(c, err)
	}

	// Update user avatar in database
	avatarURL := fmt.Sprintf("/api/avatars/%s", filename)
	if err := models.UpdateUserAvatar(userName, avatarURL); err != nil {
		// Clean up file if DB update fails
		os.Remove(filepath)
		return handleError(c, err)
	}

	// Return success response for HTMX
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Trigger", `{"avatarUpdated": {"message": "Avatar updated successfully", "status": "success"}}`)
		return c.SendString("")
	}

	return c.Redirect("/account", fiber.StatusSeeOther)
}

// generateCodeVerifier generates a random code verifier for PKCE
func generateCodeVerifier() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes)
}