package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"strings"
)

// HandleUsers renders the user administration view.
func HandleUsers(c *fiber.Ctx) error {
	users, err := models.GetUsers()
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.Users(users))
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

// filterMangasByTags filters a slice of mangas by selected tags
// tagMode can be "all" (all tags must match) or "any" (at least one tag must match)
func filterMangasByTags(mangas []models.Manga, selectedTags []string, tagMode string) []models.Manga {
	if len(selectedTags) == 0 {
		return mangas
	}

	var filtered []models.Manga
	for _, manga := range mangas {
		mangaTags, err := models.GetTagsForManga(manga.Slug)
		if err != nil {
			continue
		}

		if tagMode == "any" {
			// At least one selected tag must be in manga's tags
			for _, selTag := range selectedTags {
				for _, mTag := range mangaTags {
					if strings.EqualFold(selTag, mTag) {
						filtered = append(filtered, manga)
						goto nextManga
					}
				}
			}
		} else {
			// All selected tags must be in manga's tags
			matchCount := 0
			for _, selTag := range selectedTags {
				for _, mTag := range mangaTags {
					if strings.EqualFold(selTag, mTag) {
						matchCount++
						break
					}
				}
			}
			if matchCount == len(selectedTags) {
				filtered = append(filtered, manga)
			}
		}
	nextManga:
	}
	return filtered
}

// filterMangasBySearch filters a slice of mangas by search term using very lenient fuzzy matching
func filterMangasBySearch(mangas []models.Manga, searchTerm string) []models.Manga {
	if searchTerm == "" {
		return mangas
	}

	// Aggressive normalization function
	normalize := func(s string) string {
		s = strings.ToLower(s)
		// Remove all non-alphanumeric characters except spaces
		var result strings.Builder
		for _, r := range s {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
				result.WriteRune(r)
			} else if r >= 'A' && r <= 'Z' {
				result.WriteRune(r + 32) // Convert to lowercase
			} else {
				// Replace any other character with space
				result.WriteRune(' ')
			}
		}
		return result.String()
	}

	// Normalize and split search term
	normalizedSearch := normalize(searchTerm)
	searchWords := strings.Fields(normalizedSearch)
	if len(searchWords) == 0 {
		return mangas
	}

	var filtered []models.Manga
	for _, manga := range mangas {
		// Normalize manga name
		normalizedName := normalize(manga.Name)
		
		// Check if all search words match
		matched := true
		for _, searchWord := range searchWords {
			if searchWord == "" {
				continue
			}
			
			// First check: simple substring match
			if strings.Contains(normalizedName, searchWord) {
				continue
			}
			
			// Second check: word prefix match
			nameWords := strings.Fields(normalizedName)
			wordMatched := false
			for _, nameWord := range nameWords {
				if strings.HasPrefix(nameWord, searchWord) {
					wordMatched = true
					break
				}
				// Also check if the search word appears within the name word (substring)
				if strings.Contains(nameWord, searchWord) {
					wordMatched = true
					break
				}
			}
			
			if !wordMatched {
				matched = false
				break
			}
		}
		
		if matched {
			filtered = append(filtered, manga)
		}
	}
	return filtered
}

// HandleAccountFavorites shows paginated favorites for the current user
func HandleAccountFavorites(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	favSlugs, err := models.GetFavoritesForUser(userName)
	if err != nil {
		return handleError(c, err)
	}

	params := ParseQueryParams(c)
	pageSize := 16
	
	// Build full manga list first
	var allMangas []models.Manga
	for _, slug := range favSlugs {
		if m, err := models.GetManga(slug); err == nil && m != nil {
			allMangas = append(allMangas, *m)
		}
	}
	
	// Filter by tags if specified
	if len(params.Tags) > 0 {
		allMangas = filterMangasByTags(allMangas, params.Tags, params.TagMode)
	}
	
	// Filter by search term if specified
	if params.SearchFilter != "" {
		allMangas = filterMangasBySearch(allMangas, params.SearchFilter)
	}
	
	// Sort mangas
	models.SortMangas(allMangas, params.Sort, params.Order)
	
	// Paginate
	total := len(allMangas)
	start := (params.Page-1)*pageSize
	end := start + pageSize
	if start > len(allMangas) { start = len(allMangas) }
	if end > len(allMangas) { end = len(allMangas) }
	mangas := allMangas[start:end]
	totalPages := CalculateTotalPages(int64(total), pageSize)
	
	// Fetch all tags for user's favorites
	allTags, tagsErr := models.GetTagsForUserFavorites(userName)
	if tagsErr != nil {
		return handleError(c, tagsErr)
	}
	
	// HTMX fragment support
	if IsHTMXRequest(c) && GetHTMXTarget(c) == "account-manga-list" {
		return HandleView(c, views.AccountMangaListingWithTags(mangas, params.Page, totalPages, params.Sort, params.Order, "/account/favorites", "You have no favorites yet.", params.Tags, params.TagMode, allTags, params.SearchFilter))
	}
	return HandleView(c, views.AccountFavoritesWithTags(mangas, params.Page, totalPages, params.Sort, params.Order, params.Tags, params.TagMode, allTags, params.SearchFilter))
}

// HandleAccountUpvoted shows paginated upvoted mangas for the current user
func HandleAccountUpvoted(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	slugs, err := models.GetUpvotedMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	
	params := ParseQueryParams(c)
	pageSize := 16
	
	var allMangas []models.Manga
	for _, slug := range slugs { 
		if m, err := models.GetManga(slug); err == nil && m != nil { 
			allMangas = append(allMangas, *m) 
		} 
	}
	
	// Filter by tags if specified
	if len(params.Tags) > 0 {
		allMangas = filterMangasByTags(allMangas, params.Tags, params.TagMode)
	}
	
	// Filter by search term if specified
	if params.SearchFilter != "" {
		allMangas = filterMangasBySearch(allMangas, params.SearchFilter)
	}
	
	models.SortMangas(allMangas, params.Sort, params.Order)
	
	total := len(allMangas)
	start := (params.Page-1)*pageSize
	end := start + pageSize
	if start > len(allMangas) { start = len(allMangas) }
	if end > len(allMangas) { end = len(allMangas) }
	mangas := allMangas[start:end]
	totalPages := CalculateTotalPages(int64(total), pageSize)
	
	// Fetch all tags for user's upvoted mangas
	allTags, tagsErr := models.GetTagsForUserUpvoted(userName)
	if tagsErr != nil {
		return handleError(c, tagsErr)
	}
	
	if IsHTMXRequest(c) && GetHTMXTarget(c) == "account-manga-list" {
		return HandleView(c, views.AccountMangaListingWithTags(mangas, params.Page, totalPages, params.Sort, params.Order, "/account/upvoted", "You have not upvoted any mangas yet.", params.Tags, params.TagMode, allTags, params.SearchFilter))
	}
	return HandleView(c, views.AccountUpvotedWithTags(mangas, params.Page, totalPages, params.Sort, params.Order, params.Tags, params.TagMode, allTags, params.SearchFilter))
}

// HandleAccountReading shows paginated reading list for the current user
func HandleAccountReading(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	slugs, err := models.GetReadingMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	
	params := ParseQueryParams(c)
	pageSize := 16
	
	var allMangas []models.Manga
	for _, slug := range slugs { 
		if m, err := models.GetManga(slug); err == nil && m != nil { 
			allMangas = append(allMangas, *m) 
		} 
	}
	
	// Filter by tags if specified
	if len(params.Tags) > 0 {
		allMangas = filterMangasByTags(allMangas, params.Tags, params.TagMode)
	}
	
	// Filter by search term if specified
	if params.SearchFilter != "" {
		allMangas = filterMangasBySearch(allMangas, params.SearchFilter)
	}
	
	models.SortMangas(allMangas, params.Sort, params.Order)
	
	total := len(allMangas)
	start := (params.Page-1)*pageSize
	end := start + pageSize
	if start > len(allMangas) { start = len(allMangas) }
	if end > len(allMangas) { end = len(allMangas) }
	mangas := allMangas[start:end]
	totalPages := CalculateTotalPages(int64(total), pageSize)
	
	// Fetch all tags for user's reading mangas
	allTags, tagsErr := models.GetTagsForUserReading(userName)
	if tagsErr != nil {
		return handleError(c, tagsErr)
	}
	
	if IsHTMXRequest(c) && GetHTMXTarget(c) == "account-manga-list" {
		return HandleView(c, views.AccountMangaListingWithTags(mangas, params.Page, totalPages, params.Sort, params.Order, "/account/reading", "You are not reading any mangas right now.", params.Tags, params.TagMode, allTags, params.SearchFilter))
	}
	return HandleView(c, views.AccountReadingWithTags(mangas, params.Page, totalPages, params.Sort, params.Order, params.Tags, params.TagMode, allTags, params.SearchFilter))
}

// HandleAccountDownvoted shows paginated downvoted mangas for the current user
func HandleAccountDownvoted(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	slugs, err := models.GetDownvotedMangasForUser(userName)
	if err != nil {
		return handleError(c, err)
	}
	
	params := ParseQueryParams(c)
	pageSize := 16
	
	var allMangas []models.Manga
	for _, slug := range slugs { 
		if m, err := models.GetManga(slug); err == nil && m != nil { 
			allMangas = append(allMangas, *m) 
		} 
	}
	
	// Filter by tags if specified
	if len(params.Tags) > 0 {
		allMangas = filterMangasByTags(allMangas, params.Tags, params.TagMode)
	}
	
	// Filter by search term if specified
	if params.SearchFilter != "" {
		allMangas = filterMangasBySearch(allMangas, params.SearchFilter)
	}
	
	models.SortMangas(allMangas, params.Sort, params.Order)
	
	total := len(allMangas)
	start := (params.Page-1)*pageSize
	end := start + pageSize
	if start > len(allMangas) { start = len(allMangas) }
	if end > len(allMangas) { end = len(allMangas) }
	mangas := allMangas[start:end]
	totalPages := CalculateTotalPages(int64(total), pageSize)
	
	// Fetch all tags for user's downvoted mangas
	allTags, tagsErr := models.GetTagsForUserDownvoted(userName)
	if tagsErr != nil {
		return handleError(c, tagsErr)
	}
	
	if IsHTMXRequest(c) && GetHTMXTarget(c) == "account-manga-list" {
		return HandleView(c, views.AccountMangaListingWithTags(mangas, params.Page, totalPages, params.Sort, params.Order, "/account/downvoted", "You have not downvoted any mangas yet.", params.Tags, params.TagMode, allTags, params.SearchFilter))
	}
	return HandleView(c, views.AccountDownvotedWithTags(mangas, params.Page, totalPages, params.Sort, params.Order, params.Tags, params.TagMode, allTags, params.SearchFilter))
}
