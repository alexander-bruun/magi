package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

const (
	defaultPage     = 1
	defaultPageSize = 16
	searchPageSize  = 10
)

// HandleMangas lists mangas with filtering, sorting, and HTMX fragment support.
func HandleMangas(c *fiber.Ctx) error {
	params := ParseQueryParams(c)

	// Search mangas using options (supports tags, tagMode, and types)
	opts := models.SearchOptions{
		Filter:      params.SearchFilter,
		Page:        params.Page,
		PageSize:    defaultPageSize,
		SortBy:      params.Sort,
		SortOrder:   params.Order,
		LibrarySlug: params.LibrarySlug,
		Tags:        params.Tags,
		TagMode:     params.TagMode,
		Types:       params.Types,
	}
	mangas, count, err := models.SearchMangasWithOptions(opts)

	if err != nil {
		return handleError(c, err)
	}

	totalPages := CalculateTotalPages(count, defaultPageSize)

	// Fetch all known tags for the dropdown
	allTags, err := models.GetAllTags()
	if err != nil {
		return handleError(c, err)
	}
	// Fetch all known types for the new types dropdown
	allTypes, err := models.GetAllMangaTypes()
	if err != nil {
		return handleError(c, err)
	}

	// If HTMX request targeting the listing container, render just the generic listing
	if IsHTMXRequest(c) && GetHTMXTarget(c) == "manga-listing" {
		return HandleView(c, views.GenericMangaListingWithTypes("/mangas", "manga-listing", true, mangas, params.Page, totalPages, params.Sort, params.Order, "No mangas have been indexed yet.", params.Tags, params.TagMode, allTags, params.Types, allTypes, params.SearchFilter))
	}

	return HandleView(c, views.MangasWithTypes(mangas, params.Page, totalPages, params.Sort, params.Order, params.Tags, params.TagMode, allTags, params.Types, allTypes, params.SearchFilter))
}

// HandleManga renders a manga detail page including chapters and per-user state.
func HandleManga(c *fiber.Ctx) error {
	slug := c.Params("manga")
	manga, err := models.GetManga(slug)
	if err != nil {
		return handleError(c, err)
	}
	if manga == nil {
		return HandleView(c, views.Error("Manga not found or access restricted based on content rating settings."))
	}
	chapters, err := models.GetChapters(slug)
	if err != nil {
		return handleError(c, err)
	}
	
	// Get user role for conditional rendering
	userRole := ""
	userName := GetUserContext(c)
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil {
			userRole = user.Role
		}
		// If a user is logged in, fetch their read chapters and annotate the list
		readMap, err := models.GetReadChaptersForUser(userName, slug)
		if err == nil {
			for i := range chapters {
				chapters[i].Read = readMap[chapters[i].Slug]
			}
		}
	}
	
	// Precompute first/last chapter slugs and count for the view
	firstSlug, lastSlug := models.GetFirstAndLastChapterSlugs(chapters)
	
	return HandleView(c, views.Manga(*manga, chapters, firstSlug, lastSlug, len(chapters), userRole))
}

// HandleChapter shows a chapter reader with navigation and optional read tracking.
func HandleChapter(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	chapterSlug := c.Params("chapter")

	manga, chapters, err := models.GetMangaAndChapters(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		return handleError(c, err)
	}

	// Note: chapter is normally marked read by an HTMX trigger in the view.
	// As a safe fallback, if this request is a full page load (not an HTMX request)
	// and the user is logged in, mark the chapter read server-side so the
	// manga list can reflect the read state for non-HTMX navigation.
	if userName := GetUserContext(c); userName != "" && !IsHTMXRequest(c) {
		_ = models.MarkChapterRead(userName, mangaSlug, chapterSlug)
	}

	prevSlug, nextSlug, err := models.GetAdjacentChapters(chapter.Slug, mangaSlug)
	if err != nil {
		return handleError(c, err)
	}

	images, err := models.GetChapterImages(manga, chapter)
	if err != nil {
		return handleError(c, err)
	}

	// Provide chapters in reverse order for dropdown (newest first) to avoid view-side reversing
	rev := make([]models.Chapter, len(chapters))
	for i := range chapters {
		rev[i] = chapters[len(chapters)-1-i]
	}
	return HandleView(c, views.Chapter(prevSlug, chapter.Slug, nextSlug, *manga, images, *chapter, rev))
}

// HandleMarkRead marks a chapter as read for the logged-in user via HTMX
func HandleMarkRead(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	chapterSlug := c.Params("chapter")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}
	if err := models.MarkChapterRead(userName, mangaSlug, chapterSlug); err != nil {
		return handleError(c, err)
	}
	// Return the inline eye toggle fragment so HTMX will swap the icon in-place.
	return HandleView(c, views.InlineEyeToggle(true, mangaSlug, chapterSlug))
}

// HandleMarkUnread unmarks a chapter as read for the logged-in user via HTMX
func HandleMarkUnread(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	chapterSlug := c.Params("chapter")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}
	if err := models.UnmarkChapterRead(userName, mangaSlug, chapterSlug); err != nil {
		return handleError(c, err)
	}
	// Return the inline eye toggle fragment with read=false so HTMX swaps to the closed-eye.
	return HandleView(c, views.InlineEyeToggle(false, mangaSlug, chapterSlug))
}

// HandleUpdateMetadataManga displays Mangadex matches for updating a local manga's metadata.
func HandleUpdateMetadataManga(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	search := c.Query("search")

	response, err := models.GetMangadexMangas(search)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UpdateMetadata(*response, mangaSlug))
}

// HandleEditMetadataManga applies selected Mangadex metadata to an existing manga.
func HandleEditMetadataManga(c *fiber.Ctx) error {
	mangadexID := c.Query("id")
	mangaSlug := c.Query("slug")
	if mangaSlug == "" {
		return handleError(c, fmt.Errorf("manga slug can't be empty"))
	}

	existingManga, err := models.GetManga(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if existingManga == nil {
		return handleError(c, fmt.Errorf("manga not found or access restricted"))
	}

	mangaDetail, err := models.GetMangadexManga(mangadexID)
	if err != nil {
		return handleError(c, err)
	}

	coverArtURL, err := models.ExtractCoverArtURL(mangaDetail, mangadexID)
	if err != nil {
		return handleError(c, err)
	}

	cachedImageURL, err := models.CacheAndGetImageURL(savedCacheDirectory, existingManga.Slug, coverArtURL)
	if err != nil {
		return handleError(c, err)
	}

	models.UpdateMangaFromMangadex(existingManga, mangaDetail, cachedImageURL)

	// Persist tags from Mangadex
	if err := models.PersistMangadexTags(existingManga.Slug, mangaDetail); err != nil {
		return handleError(c, err)
	}

	if err := models.UpdateManga(existingManga); err != nil {
		return handleError(c, err)
	}

	redirectURL := fmt.Sprintf("/mangas/%s", existingManga.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

// HandleMangaSearch returns search results for the quick-search panel.
func HandleMangaSearch(c *fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		return HandleView(c, views.OneDoesNotSimplySearch())
	}

	mangas, _, err := models.SearchMangas(searchParam, defaultPage, searchPageSize, "name", "desc", "", "")
	if err != nil {
		return handleError(c, err)
	}

	if len(mangas) == 0 {
		return HandleView(c, views.NoResultsSearch())
	}

	return HandleView(c, views.SearchMangas(mangas))
}

// HandleTags returns a JSON array of all known tags for client-side consumption
func HandleTags(c *fiber.Ctx) error {
	tags, err := models.GetAllTags()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tags)
}

// HandleTagsFragment returns an HTMX-ready fragment with tag checkboxes
func HandleTagsFragment(c *fiber.Ctx) error {
	tags, err := models.GetAllTags()
	if err != nil {
		return handleError(c, err)
	}

	// Determine currently selected tags from the query (support repeated and comma-separated)
	var selectedTags []string
	if raw := string(c.Request().URI().QueryString()); raw != "" {
		if valsMap, err := url.ParseQuery(raw); err == nil {
			if vals, ok := valsMap["tags"]; ok {
				for _, v := range vals {
					for _, t := range strings.Split(v, ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							selectedTags = append(selectedTags, t)
						}
					}
				}
			}
		}
	}
	// Render fragment directly without layout wrapper
	return renderComponent(c, views.TagsFragment(tags, selectedTags))
}

// templEscape provides a minimal HTML escape for values inserted into the fragment
func templEscape(s string) string {
	r := s
	r = strings.ReplaceAll(r, "&", "&amp;")
	r = strings.ReplaceAll(r, "<", "&lt;")
	r = strings.ReplaceAll(r, ">", "&gt;")
	r = strings.ReplaceAll(r, "\"", "&quot;")
	return r
}

// HandleMangaVote handles a user's upvote/downvote for a manga via HTMX.
// Expected form values: "value" = "1" or "-1". User must be authenticated.
func HandleMangaVote(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// parse value
	valStr := c.FormValue("value")
	if valStr == "" {
		return fiber.ErrBadRequest
	}
	v, err := strconv.Atoi(valStr)
	if err != nil {
		return fiber.ErrBadRequest
	}

	// If value == 0, remove vote
	if v == 0 {
		if err := models.RemoveVote(userName, mangaSlug); err != nil {
			return handleError(c, err)
		}
	} else {
		if err := models.SetVote(userName, mangaSlug, v); err != nil {
			return handleError(c, err)
		}
	}

	// Return updated fragment so HTMX can refresh the vote UI in-place.
	score, up, down, err := models.GetMangaVotes(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	userVote, _ := models.GetUserVoteForManga(userName, mangaSlug)
	return HandleView(c, views.MangaVoteFragment(mangaSlug, score, up, down, userVote))
}

// HandleMangaVoteFragment returns the vote UI fragment for a manga. If user is logged in,
// it will show their current selection highlighted.
func HandleMangaVoteFragment(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	userName := GetUserContext(c)
	score, up, down, err := models.GetMangaVotes(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	userVote := 0
	if userName != "" {
		v, _ := models.GetUserVoteForManga(userName, mangaSlug)
		userVote = v
	}
	return HandleView(c, views.MangaVoteFragment(mangaSlug, score, up, down, userVote))
}

// HandleMangaFavorite handles toggling a favorite for the logged-in user via HTMX.
// Expected form values: "value" = "1" to favorite or "0" to unfavorite.
func HandleMangaFavorite(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	valStr := c.FormValue("value")
	if valStr == "" {
		return fiber.ErrBadRequest
	}

	if valStr == "0" {
		if err := models.RemoveFavorite(userName, mangaSlug); err != nil {
			return handleError(c, err)
		}
	} else {
		if err := models.SetFavorite(userName, mangaSlug); err != nil {
			return handleError(c, err)
		}
	}

	// Return updated fragment so HTMX can refresh the favorite UI in-place.
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	isFav, _ := models.IsFavoriteForUser(userName, mangaSlug)
	return HandleView(c, views.MangaFavoriteFragment(mangaSlug, favCount, isFav))
}

// HandleMangaFavoriteFragment returns the favorite UI fragment for a manga.
func HandleMangaFavoriteFragment(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	userName := GetUserContext(c)
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	isFav := false
	if userName != "" {
		f, _ := models.IsFavoriteForUser(userName, mangaSlug)
		isFav = f
	}
	return HandleView(c, views.MangaFavoriteFragment(mangaSlug, favCount, isFav))
}

// HandleManualEditMetadata handles manual metadata updates by moderators or admins
func HandleManualEditMetadata(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	if mangaSlug == "" {
		return handleError(c, fmt.Errorf("manga slug can't be empty"))
	}

	existingManga, err := models.GetMangaUnfiltered(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if existingManga == nil {
		return handleError(c, fmt.Errorf("manga not found"))
	}

	// Parse form values
	name := c.FormValue("name")
	author := c.FormValue("author")
	description := c.FormValue("description")
	year := c.FormValue("year")
	originalLanguage := c.FormValue("original_language")
	mangaType := c.FormValue("manga_type")
	status := c.FormValue("status")
	contentRating := c.FormValue("content_rating")
	tagsInput := c.FormValue("tags")
	coverURL := c.FormValue("cover_url")

	// Update fields
	if name != "" {
		existingManga.Name = name
	}
	if author != "" {
		existingManga.Author = author
	}
	if description != "" {
		existingManga.Description = description
	}
	if year != "" {
		if yearInt, err := strconv.Atoi(year); err == nil {
			existingManga.Year = yearInt
		}
	}
	if originalLanguage != "" {
		existingManga.OriginalLanguage = originalLanguage
	}
	if mangaType != "" {
		existingManga.Type = mangaType
	}
	if status != "" {
		existingManga.Status = status
	}
	if contentRating != "" {
		existingManga.ContentRating = contentRating
	}

	// Process tags (comma-separated list)
	if tagsInput != "" {
		var tags []string
		for _, tag := range strings.Split(tagsInput, ",") {
			trimmed := strings.TrimSpace(tag)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
		if len(tags) > 0 {
			if err := models.SetTagsForManga(existingManga.Slug, tags); err != nil {
				return handleError(c, fmt.Errorf("failed to update tags: %w", err))
			}
		}
	}

	// Process cover art URL (download and cache)
	if coverURL != "" {
		cachedImageURL, err := models.CacheAndGetImageURL(savedCacheDirectory, existingManga.Slug, coverURL)
		if err != nil {
			return handleError(c, fmt.Errorf("failed to download and cache cover art: %w", err))
		}
		existingManga.CoverArtURL = cachedImageURL
	}

	// Update manga in database
	if err := models.UpdateManga(existingManga); err != nil {
		return handleError(c, err)
	}

	// Return success response for HTMX
	redirectURL := fmt.Sprintf("/mangas/%s", existingManga.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

// HandleRefreshMetadata re-indexes a single manga by rescanning its folder
func HandleRefreshMetadata(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	if mangaSlug == "" {
		return handleError(c, fmt.Errorf("manga slug can't be empty"))
	}

	existingManga, err := models.GetMangaUnfiltered(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if existingManga == nil {
		return handleError(c, fmt.Errorf("manga not found"))
	}

	// Store the path and library slug before deletion
	mangaPath := existingManga.Path
	librarySlug := existingManga.LibrarySlug

	// Delete the existing manga to force a fresh index
	// This will also delete associated chapters, tags, etc.
	if err := models.DeleteManga(mangaSlug); err != nil {
		return handleError(c, fmt.Errorf("failed to delete existing manga: %w", err))
	}

	// Re-index the manga from scratch
	newSlug, err := indexer.IndexManga(mangaPath, librarySlug)
	if err != nil {
		return handleError(c, fmt.Errorf("failed to refresh metadata: %w", err))
	}
	
	if newSlug == "" {
		return handleError(c, fmt.Errorf("manga indexing returned empty slug"))
	}

	// Return success response for HTMX
	redirectURL := fmt.Sprintf("/mangas/%s", newSlug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}
