package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"strconv"
)

func HandleUsers(c *fiber.Ctx) error {
	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.Users(users))
}

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

func HandleUserUnban(c *fiber.Ctx) error {
	username := c.Params("username")

	models.UnbanUser(username)

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

func HandleUserPromote(c *fiber.Ctx) error {
	username := c.Params("username")

	models.PromoteUser(username)

	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UsersTable(users))
}

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
	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// Load user profile
	user, err := models.FindUserByUsername(userName)
	if err != nil {
		return handleError(c, err)
	}
	if user == nil {
		return fiber.ErrNotFound
	}

	// Load favorite mangas for the user
	favSlugs, err := models.GetFavoritesForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	var favorites []models.Manga
	for i, slug := range favSlugs {
		if i >= 10 { // limit carousel to 10
			break
		}
		if m, err := models.GetManga(slug); err == nil && m != nil {
			favorites = append(favorites, *m)
		}
	}

	// Load reading states (mangas the user has any read chapters for)
	readingSlugs, err := models.GetReadingMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	var reading []models.Manga
	for i, slug := range readingSlugs {
		if i >= 10 {
			break
		}
		if m, err := models.GetManga(slug); err == nil && m != nil {
			reading = append(reading, *m)
		}
	}

	// Load liked mangas (user upvoted)
	likedSlugs, err := models.GetUpvotedMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	var liked []models.Manga
	for i, slug := range likedSlugs {
		if i >= 10 {
			break
		}
		if m, err := models.GetManga(slug); err == nil && m != nil {
			liked = append(liked, *m)
		}
	}

	// Load downvoted mangas (user downvoted)
	downvotedSlugs, err := models.GetDownvotedMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	var downvoted []models.Manga
	for i, slug := range downvotedSlugs {
		if i >= 10 {
			break
		}
		if m, err := models.GetManga(slug); err == nil && m != nil {
			downvoted = append(downvoted, *m)
		}
	}
	return HandleView(c, views.Account(*user, userName, favorites, reading, liked, downvoted))
}

// Helpers for paginated account lists
func slicePage(slugs []string, page, pageSize int) (start, end int) {
	total := len(slugs)
	if page < 1 {
		page = 1
	}
	start = (page - 1) * pageSize
	if start > total {
		start = total
	}
	end = start + pageSize
	if end > total {
		end = total
	}
	return
}

// HandleAccountFavorites shows paginated favorites for the current user
func HandleAccountFavorites(c *fiber.Ctx) error {
	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	favSlugs, err := models.GetFavoritesForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	total := len(favSlugs)

	page := 1
	if p := c.Query("page", "1"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	pageSize := 16
	start, end := slicePage(favSlugs, page, pageSize)

	var mangas []models.Manga
	for _, slug := range favSlugs[start:end] {
		if m, err := models.GetManga(slug); err == nil && m != nil {
			mangas = append(mangas, *m)
		}
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	return HandleView(c, views.AccountFavorites(mangas, page, totalPages))
}

// HandleAccountUpvoted shows paginated upvoted mangas for the current user
func HandleAccountUpvoted(c *fiber.Ctx) error {
	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	slugs, err := models.GetUpvotedMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	total := len(slugs)
	page := 1
	if p := c.Query("page", "1"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	pageSize := 16
	start, end := slicePage(slugs, page, pageSize)

	var mangas []models.Manga
	for _, slug := range slugs[start:end] {
		if m, err := models.GetManga(slug); err == nil && m != nil {
			mangas = append(mangas, *m)
		}
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	return HandleView(c, views.AccountUpvoted(mangas, page, totalPages))
}

// HandleAccountReading shows paginated reading list for the current user
func HandleAccountReading(c *fiber.Ctx) error {
	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	slugs, err := models.GetReadingMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	total := len(slugs)
	page := 1
	if p := c.Query("page", "1"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	pageSize := 16
	start, end := slicePage(slugs, page, pageSize)

	var mangas []models.Manga
	for _, slug := range slugs[start:end] {
		if m, err := models.GetManga(slug); err == nil && m != nil {
			mangas = append(mangas, *m)
		}
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	return HandleView(c, views.AccountReading(mangas, page, totalPages))
}

// HandleAccountDownvoted shows paginated downvoted mangas for the current user
func HandleAccountDownvoted(c *fiber.Ctx) error {
	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	slugs, err := models.GetDownvotedMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	total := len(slugs)
	page := 1
	if p := c.Query("page", "1"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	pageSize := 16
	start, end := slicePage(slugs, page, pageSize)

	var mangas []models.Manga
	for _, slug := range slugs[start:end] {
		if m, err := models.GetManga(slug); err == nil && m != nil {
			mangas = append(mangas, *m)
		}
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	// The downvoted template was generated with the name AccountDownvoted (contains "Your Downvoted"), so render that.
	return HandleView(c, views.AccountDownvoted(mangas, page, totalPages))
}
