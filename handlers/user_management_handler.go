package handlers

import (
	"fmt"

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