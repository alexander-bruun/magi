package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

// AccountListType represents the type of account list
type AccountListType string

const (
	AccountListFavorites AccountListType = "favorites"
	AccountListUpvoted   AccountListType = "upvoted"
	AccountListDownvoted AccountListType = "downvoted"
	AccountListReading   AccountListType = "reading"
)

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

// accountListConfig defines the configuration for each account list type
type accountListConfig struct {
	listType      AccountListType
	getMediasFunc func(models.UserMediaListOptions) ([]models.Media, int, error)
	getTagsFunc   func(string) ([]string, error)
	emptyMessage  string
	path          string
}

// getAccountListConfig returns the configuration for a given list type
func getAccountListConfig(listType AccountListType) accountListConfig {
	configs := map[AccountListType]accountListConfig{
		AccountListFavorites: {
			listType:      AccountListFavorites,
			getMediasFunc: models.GetUserFavoritesWithOptions,
			getTagsFunc:   models.GetTagsForUserFavorites,
			emptyMessage:  "You have no favorites yet.",
			path:          "/account/favorites",
		},
		AccountListUpvoted: {
			listType:        AccountListUpvoted,
			getMediasFunc:   models.GetUserUpvotedWithOptions,
			getTagsFunc:     models.GetTagsForUserUpvoted,
			emptyMessage:    "You have not upvoted any media yet.",
			path:            "/account/upvoted",
		},
		AccountListDownvoted: {
			listType:        AccountListDownvoted,
			getMediasFunc:   models.GetUserDownvotedWithOptions,
			getTagsFunc:     models.GetTagsForUserDownvoted,
			emptyMessage:    "You have not downvoted any media yet.",
			path:            "/account/downvoted",
		},
		AccountListReading: {
			listType:      AccountListReading,
			getMediasFunc: models.GetUserReadingWithOptions,
			getTagsFunc:   models.GetTagsForUserReading,
			emptyMessage:  "You are not reading any media right now.",
			path:          "/account/reading",
		},
	}
	return configs[listType]
}

// HandleAccountList is the unified handler for all account media lists
func HandleAccountList(listType AccountListType) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userName := GetUserContext(c)
		if userName == "" {
			return fiber.ErrUnauthorized
		}

		config := getAccountListConfig(listType)
		params := ParseQueryParams(c)
		pageSize := 16

		// Get accessible libraries for the current user
		accessibleLibraries, err := GetUserAccessibleLibraries(c)
		if err != nil {
			return handleError(c, err)
		}

		opts := models.UserMediaListOptions{
			Username:            userName,
			Page:                params.Page,
			PageSize:            pageSize,
			SortBy:              params.Sort,
			SortOrder:           params.Order,
			Tags:                params.Tags,
			TagMode:             params.TagMode,
			SearchFilter:        params.SearchFilter,
			AccessibleLibraries: accessibleLibraries,
		}

		media, total, err := config.getMediasFunc(opts)
		if err != nil {
			return handleError(c, err)
		}

		totalPages := CalculateTotalPages(int64(total), pageSize)

		allTags, tagsErr := config.getTagsFunc(userName)
		if tagsErr != nil {
			return handleError(c, tagsErr)
		}

		// HTMX fragment support
		if IsHTMXRequest(c) {
			target := GetHTMXTarget(c)
			if target == "account-listing" {
				return HandleView(c, views.AccountMediaListingWithTags(
					media, params.Page, totalPages, params.Sort, params.Order,
					config.path, config.emptyMessage, params.Tags, params.TagMode, allTags, params.SearchFilter,
				))
			} else if target == "account-media-list-results" {
				return HandleView(c, views.MediaListingFragment(
					media, params.Page, totalPages, params.Sort, params.Order,
					config.emptyMessage, config.path, "account-media-list-results",
					params.Tags, params.TagMode, nil, params.SearchFilter,
				))
			}
		}

		// All views now call the standard AccountPageLayout function
		title, breadcrumbLabel, _, _ := GetAccountListConfig(string(listType))
		return HandleView(c, views.AccountPageLayout(
			title, breadcrumbLabel, config.path, media, params.Page, totalPages, params.Sort, params.Order,
			config.emptyMessage, allTags, params.Tags, params.TagMode, params.SearchFilter,
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

	models.UpdateUserRole(username, "reader")
	models.BanUser(username)

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

	models.PromoteUser(username)

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

// HandleUserDemote reduces a user's role and refreshes the table view.
func HandleUserDemote(c *fiber.Ctx) error {
	username := c.Params("username")

	models.DemoteUser(username)

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