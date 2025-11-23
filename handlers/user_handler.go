package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

// accountListConfig defines the configuration for each account list type
type accountListConfig struct {
	listType        views.AccountListType
	getMangasFunc   func(models.UserMangaListOptions) ([]models.Manga, int, error)
	getLightNovelsFunc func(models.UserLightNovelListOptions) ([]models.LightNovel, int, error)
	getTagsFunc     func(string) ([]string, error)
	emptyMessage    string
	path            string
	isLightNovel    bool
}

// getAccountListConfig returns the configuration for a given list type
func getAccountListConfig(listType views.AccountListType) accountListConfig {
	configs := map[views.AccountListType]accountListConfig{
		views.AccountListFavorites: {
			listType:      views.AccountListFavorites,
			getMangasFunc: models.GetUserFavoritesWithOptions,
			getTagsFunc:   models.GetTagsForUserFavorites,
			emptyMessage:  "You have no favorites yet.",
			path:          "/account/favorites",
			isLightNovel:  false,
		},
		views.AccountListUpvoted: {
			listType:        views.AccountListUpvoted,
			getMangasFunc:   models.GetUserUpvotedWithOptions,
			getTagsFunc:     models.GetTagsForUserUpvoted,
			emptyMessage:    "You have not upvoted any mangas yet.",
			path:            "/account/upvoted",
			isLightNovel:    false,
		},
		views.AccountListDownvoted: {
			listType:        views.AccountListDownvoted,
			getMangasFunc:   models.GetUserDownvotedWithOptions,
			getTagsFunc:     models.GetTagsForUserDownvoted,
			emptyMessage:    "You have not downvoted any mangas yet.",
			path:            "/account/downvoted",
			isLightNovel:    false,
		},
		views.AccountListReading: {
			listType:      views.AccountListReading,
			getMangasFunc: models.GetUserReadingWithOptions,
			getTagsFunc:   models.GetTagsForUserReading,
			emptyMessage:  "You are not reading any mangas right now.",
			path:          "/account/reading",
			isLightNovel:  false,
		},
		views.AccountListLightNovelFavorites: {
			listType:            views.AccountListLightNovelFavorites,
			getLightNovelsFunc:  models.GetUserLightNovelFavoritesWithOptions,
			getTagsFunc:         models.GetTagsForUserLightNovelFavorites,
			emptyMessage:        "You have no favorite light novels yet.",
			path:                "/account/light-novel-favorites",
			isLightNovel:        true,
		},
		views.AccountListLightNovelUpvoted: {
			listType:            views.AccountListLightNovelUpvoted,
			getLightNovelsFunc:  models.GetUserLightNovelUpvotedWithOptions,
			getTagsFunc:         models.GetTagsForUserLightNovelUpvoted,
			emptyMessage:        "You have not upvoted any light novels yet.",
			path:                "/account/light-novel-upvoted",
			isLightNovel:        true,
		},
		views.AccountListLightNovelDownvoted: {
			listType:            views.AccountListLightNovelDownvoted,
			getLightNovelsFunc:  models.GetUserLightNovelDownvotedWithOptions,
			getTagsFunc:         models.GetTagsForUserLightNovelDownvoted,
			emptyMessage:        "You have not downvoted any light novels yet.",
			path:                "/account/light-novel-downvoted",
			isLightNovel:        true,
		},
	}
	return configs[listType]
}

// HandleAccountList is the unified handler for all account manga lists
func HandleAccountList(listType views.AccountListType) fiber.Handler {
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

		// Check if this is a light novel list type
		isLightNovel := listType == views.AccountListLightNovelFavorites || 
					   listType == views.AccountListLightNovelUpvoted || 
					   listType == views.AccountListLightNovelDownvoted

		if isLightNovel {
			opts := models.UserLightNovelListOptions{
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

			lightNovels, total, err := config.getLightNovelsFunc(opts)
			if err != nil {
				return handleError(c, err)
			}

			totalPages := CalculateTotalPages(int64(total), pageSize)

			allTags, tagsErr := config.getTagsFunc(userName)
			if tagsErr != nil {
				return handleError(c, tagsErr)
			}

			// HTMX fragment support
			if IsHTMXRequest(c) && GetHTMXTarget(c) == "account-list" {
				return HandleView(c, views.AccountLightNovelListingWithTags(
					lightNovels, params.Page, totalPages, params.Sort, params.Order,
					config.path, config.emptyMessage, params.Tags, params.TagMode, allTags, params.SearchFilter,
				))
			}

			// All views now call the single ConsolidatedAccountList function
			return HandleView(c, views.ConsolidatedAccountList(
				listType, nil, lightNovels, params.Page, totalPages, params.Sort, params.Order,
				params.Tags, params.TagMode, allTags, params.SearchFilter,
			))
		} else {
			opts := models.UserMangaListOptions{
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

			mangas, total, err := config.getMangasFunc(opts)
			if err != nil {
				return handleError(c, err)
			}

			totalPages := CalculateTotalPages(int64(total), pageSize)

			allTags, tagsErr := config.getTagsFunc(userName)
			if tagsErr != nil {
				return handleError(c, tagsErr)
			}

			// HTMX fragment support
			if IsHTMXRequest(c) && GetHTMXTarget(c) == "account-manga-list" {
				return HandleView(c, views.AccountMangaListingWithTags(
					mangas, params.Page, totalPages, params.Sort, params.Order,
					config.path, config.emptyMessage, params.Tags, params.TagMode, allTags, params.SearchFilter,
				))
			}

			// All views now call the single ConsolidatedAccountList function
			return HandleView(c, views.ConsolidatedAccountList(
				listType, mangas, nil, params.Page, totalPages, params.Sort, params.Order,
				params.Tags, params.TagMode, allTags, params.SearchFilter,
			))
		}
	}
}

// Account list route handlers
func HandleAccountFavorites(c *fiber.Ctx) error {
	return HandleAccountList(views.AccountListFavorites)(c)
}

func HandleAccountUpvoted(c *fiber.Ctx) error {
	return HandleAccountList(views.AccountListUpvoted)(c)
}

func HandleAccountDownvoted(c *fiber.Ctx) error {
	return HandleAccountList(views.AccountListDownvoted)(c)
}

func HandleAccountReading(c *fiber.Ctx) error {
	return HandleAccountList(views.AccountListReading)(c)
}

func HandleAccountLightNovelFavorites(c *fiber.Ctx) error {
	return HandleAccountList(views.AccountListLightNovelFavorites)(c)
}

func HandleAccountLightNovelUpvoted(c *fiber.Ctx) error {
	return HandleAccountList(views.AccountListLightNovelUpvoted)(c)
}

func HandleAccountLightNovelDownvoted(c *fiber.Ctx) error {
	return HandleAccountList(views.AccountListLightNovelDownvoted)(c)
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

// HandleAccount renders the current user's account page showing favorites, reading lists and liked mangas
func HandleAccount(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	return HandleView(c, views.Account(userName))
}
