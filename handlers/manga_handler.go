package handlers

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
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

	// Search mangas based on tag selection
	var mangas []models.Manga
	var count int64
	var err error

	if len(params.Tags) > 0 {
		if params.TagMode == "any" {
			mangas, count, err = models.SearchMangasWithAnyTags(params.SearchFilter, params.Page, defaultPageSize, params.Sort, params.Order, "", params.LibrarySlug, params.Tags)
		} else {
			mangas, count, err = models.SearchMangasWithTags(params.SearchFilter, params.Page, defaultPageSize, params.Sort, params.Order, "", params.LibrarySlug, params.Tags)
		}
	} else {
		mangas, count, err = models.SearchMangas(params.SearchFilter, params.Page, defaultPageSize, params.Sort, params.Order, "", params.LibrarySlug)
	}

	if err != nil {
		return handleError(c, err)
	}

	totalPages := CalculateTotalPages(count, defaultPageSize)

	// Fetch all known tags for the dropdown
	allTags, err := models.GetAllTags()
	if err != nil {
		return handleError(c, err)
	}

	// If HTMX request targeting the listing container, render just the generic listing
	if IsHTMXRequest(c) && GetHTMXTarget(c) == "manga-listing" {
		return HandleView(c, views.GenericMangaListing("/mangas", "manga-listing", true, mangas, params.Page, totalPages, params.Sort, params.Order, "No mangas have been indexed yet.", params.Tags, params.TagMode, allTags))
	}

	return HandleView(c, views.Mangas(mangas, params.Page, totalPages, params.Sort, params.Order, params.Tags, params.TagMode, allTags))
}

// HandleManga renders a manga detail page including chapters and per-user state.
func HandleManga(c *fiber.Ctx) error {
	slug := c.Params("manga")
	manga, err := models.GetManga(slug)
	if err != nil {
		return handleError(c, err)
	}
	chapters, err := models.GetChapters(slug)
	if err != nil {
		return handleError(c, err)
	}
	
	// If a user is logged in, fetch their read chapters and annotate the list
	if userName := GetUserContext(c); userName != "" {
		readMap, err := models.GetReadChaptersForUser(userName, slug)
		if err == nil {
			for i := range chapters {
				chapters[i].Read = readMap[chapters[i].Slug]
			}
		}
	}
	
	// Precompute first/last chapter slugs and count for the view
	firstSlug, lastSlug := "", ""
	if len(chapters) > 0 {
		firstSlug = chapters[0].Slug
		lastSlug = chapters[len(chapters)-1].Slug
	}
	return HandleView(c, views.Manga(*manga, chapters, firstSlug, lastSlug, len(chapters)))
}

// HandleChapter shows a chapter reader with navigation and optional read tracking.
func HandleChapter(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	chapterSlug := c.Params("chapter")

	manga, chapters, err := getMangaAndChapters(mangaSlug)
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

	images, err := getChapterImages(manga, chapter)
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
	mangaSlug := c.Params("slug")
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

	mangaDetail, err := models.GetMangadexManga(mangadexID)
	if err != nil {
		return handleError(c, err)
	}

	coverArtURL, err := extractCoverArtURL(mangaDetail, mangadexID)
	if err != nil {
		return handleError(c, err)
	}

	cachedImageURL, err := cacheAndGetImageURL(existingManga.Slug, coverArtURL)
	if err != nil {
		return handleError(c, err)
	}

	updateMangaDetails(existingManga, mangaDetail, cachedImageURL)

	// Persist tags from Mangadex
	if err := persistMangadexTags(existingManga.Slug, mangaDetail); err != nil {
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

// Helper functions

func getMangaAndChapters(mangaSlug string) (*models.Manga, []models.Chapter, error) {
	manga, err := models.GetManga(mangaSlug)
	if err != nil {
		return nil, nil, err
	}

	chapters, err := models.GetChapters(mangaSlug)
	if err != nil {
		return nil, nil, err
	}

	return manga, chapters, nil
}

func getChapterImages(manga *models.Manga, chapter *models.Chapter) ([]string, error) {
	chapterFilePath := filepath.Join(manga.Path, chapter.File)
	pageCount, err := utils.CountImageFiles(chapterFilePath)
	if err != nil {
		return nil, err
	}

	if pageCount <= 0 {
		return []string{}, nil
	}

	images := make([]string, pageCount)
	for i := 0; i < pageCount; i++ {
		images[i] = fmt.Sprintf("/api/comic?manga=%s&chapter=%s&page=%d", manga.Slug, chapter.Slug, i+1)
	}

	return images, nil
}

func extractCoverArtURL(mangaDetail *models.MangaDetail, mangadexID string) (string, error) {
	for _, rel := range mangaDetail.Relationships {
		if rel.Type == "cover_art" {
			if attributes, ok := rel.Attributes.(map[string]interface{}); ok {
				if fileName, exists := attributes["fileName"].(string); exists {
					return fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", mangadexID, fileName), nil
				}
			}
		}
	}
	return "", fmt.Errorf("cover art URL not found")
}

func cacheAndGetImageURL(slug, coverArtURL string) (string, error) {
	u, err := url.Parse(coverArtURL)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %w", err)
	}

	filename := filepath.Base(u.Path)
	fileExt := filepath.Ext(filename)[1:] // remove leading dot

	err = utils.DownloadImage(savedCacheDirectory, slug, coverArtURL)
	if err != nil {
		return "", fmt.Errorf("error downloading image: %w", err)
	}

	return fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt), nil
}

func updateMangaDetails(manga *models.Manga, mangaDetail *models.MangaDetail, coverArtURL string) {
	manga.Name = mangaDetail.Attributes.Title["en"]
	manga.Description = mangaDetail.Attributes.Description["en"]
	manga.Year = mangaDetail.Attributes.Year
	manga.OriginalLanguage = mangaDetail.Attributes.OriginalLanguage
	manga.Status = mangaDetail.Attributes.Status
	manga.ContentRating = mangaDetail.Attributes.ContentRating
	manga.CoverArtURL = coverArtURL
}

// persist tags from Mangadex metadata for a manga
func persistMangadexTags(mangaSlug string, mangaDetail *models.MangaDetail) error {
	if mangaDetail == nil || len(mangaDetail.Attributes.Tags) == 0 {
		return nil
	}
	var tags []string
	for _, t := range mangaDetail.Attributes.Tags {
		if name, ok := t.Attributes.Name["en"]; ok && name != "" {
			tags = append(tags, name)
		} else {
			for _, v := range t.Attributes.Name {
				if v != "" {
					tags = append(tags, v)
					break
				}
			}
		}
	}
	if len(tags) == 0 {
		return nil
	}
	return models.SetTagsForManga(mangaSlug, tags)
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
