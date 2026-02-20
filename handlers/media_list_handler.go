package handlers

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

const (
	defaultPage     = 1
	defaultPageSize = 16
	searchPageSize  = 10
)

// MediaListData holds data needed for rendering media lists
type MediaListData struct {
	Media       []models.Media
	TotalPages  int
	AllTags     []string
	AllTypes    []string
	SearchCount int
}

// GetMediaListData retrieves all data needed for media listing with filtering
func GetMediaListData(params QueryParams, userName string) (*MediaListData, error) {
	cfg, err := models.GetAppConfig()
	if err != nil {
		return nil, err
	}

	// Get accessible libraries for the current user
	var accessibleLibraries []string
	if userName == "" {
		// Anonymous user
		accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
	} else {
		accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
	}
	if err != nil {
		return nil, err
	}

	// Determine content rating limit and accessible libraries
	contentRatingLimit := GetContentRatingLimit(userName)
	accessibleLibs := accessibleLibraries

	// Search media using options
	opts := models.SearchOptions{
		SearchFilter:        params.SearchFilter,
		Page:                params.Page,
		PageSize:            defaultPageSize,
		SortBy:              params.Sort,
		SortOrder:           params.Order,
		LibrarySlug:         params.LibrarySlug,
		Tags:                params.Tags,
		TagMode:             params.TagMode,
		Types:               params.Types,
		AccessibleLibraries: accessibleLibs,
		ContentRatingLimit:  contentRatingLimit,
	}

	media, count, err := models.SearchMediasWithOptions(opts)
	if err != nil {
		return nil, err
	}

	totalPages := CalculateTotalPages(int64(count), defaultPageSize)

	// Enrich media with premium countdowns
	for i := range media {
		_, countdown, err := models.HasPremiumChapters(media[i].Slug, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled)
		if err != nil {
			log.Errorf("Error checking premium chapters for %s: %v", media[i].Slug, err)
		}
		media[i].PremiumCountdown = countdown
	}

	// Fetch all known tags for the dropdown
	allTags, err := models.GetAllTags()
	if err != nil {
		return nil, err
	}

	// Fetch all known types for the dropdown
	allTypes, err := models.GetAllMediaTypes()
	if err != nil {
		return nil, err
	}

	return &MediaListData{
		Media:       media,
		TotalPages:  totalPages,
		AllTags:     allTags,
		AllTypes:    allTypes,
		SearchCount: int(count),
	}, nil
}

// HandleMedias lists media with filtering, sorting, and HTMX fragment support.
func HandleMedias(c fiber.Ctx) error {
	params := ParseQueryParams(c)
	userName := GetUserContext(c)

	// Determine if user is moderator or admin
	isModerator := false
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil && (user.Role == "moderator" || user.Role == "admin") {
			isModerator = true
		}
	}

	// Restrict library filter to moderators only
	if !isModerator {
		params.LibrarySlug = ""
	}

	// Fetch all libraries if user is moderator
	var allLibraries []models.Library
	if isModerator {
		var err error
		allLibraries, err = models.GetLibraries()
		if err != nil {
			log.Errorf("Failed to get libraries: %v", err)
			// Continue without libraries
		}
	}

	data, err := GetMediaListData(params, userName)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	// If HTMX request targeting the listing results container, render just the listing fragment
	if isHTMXRequest(c) {
		target := GetHTMXTarget(c)
		switch target {
		case "media-listing":
			return handleView(c, templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				_, err := w.Write([]byte(`<div id="media-listing">`))
				if err != nil {
					return err
				}
				err = views.GenericMediaListingWithTypes("/series", "media-listing", true, data.Media, params.Page, data.TotalPages, params.Sort, params.Order, "No media have been indexed yet.", params.Tags, params.TagMode, data.AllTags, params.Types, data.AllTypes, params.SearchFilter, isModerator, allLibraries, params.LibrarySlug).Render(ctx, w)
				if err != nil {
					return err
				}
				_, err = w.Write([]byte(`</div>`))
				return err
			}))
		case "media-listing-results":
			path := "/series"
			targetID := "media-listing-results"
			emptyMessage := "No media have been indexed yet."
			return handleView(c, views.MediaListingFragment(
				data.Media,
				params.Page,
				data.TotalPages,
				params.Sort,
				params.Order,
				emptyMessage,
				path,
				targetID,
				params.Tags,
				params.TagMode,
				params.Types,
				params.SearchFilter,
			))
		}
	}

	return handleView(c, views.MediasWithTypes(data.Media, params.Page, data.TotalPages, params.Sort, params.Order, params.Tags, params.TagMode, data.AllTags, params.Types, data.AllTypes, params.SearchFilter, isModerator, allLibraries, params.LibrarySlug))
}

// HandleMedia renders a media detail page including chapters and per-user state.
func HandleMedia(c fiber.Ctx) error {
	slug := c.Params("media")
	userName := GetUserContext(c)

	// Get content rating limit based on user
	cfg, err := models.GetAppConfig()
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	contentRatingLimit := GetContentRatingLimit(userName)

	// Get media - admins can see all media, others are filtered
	media, err := models.GetMediaWithContentLimit(slug, contentRatingLimit)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if media == nil {
		log.Infof("HandleMedia: media not found for slug=%s", slug)
		return SendNotFoundError(c, ErrMediaNotFound)
	}

	// Check library access permission
	chapters, err := models.GetChapters(slug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	if err := checkMediaAccess(c, chapters); err != nil {
		return err
	}

	premiumDuration := cfg.PremiumEarlyAccessDuration

	// Parse pagination and sort params
	page, limit, sortParam := getPaginationParams(c)
	offset := (page - 1) * limit

	chapters, chapterCount, err := getChaptersData(slug, offset, limit, sortParam, cfg)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	// Get first/last chapter slugs
	firstSlug, lastSlug := getFirstLastSlugs(slug)

	// Find library slugs for first and last chapters
	firstLibrarySlug := ""
	lastLibrarySlug := ""
	for _, ch := range chapters {
		if ch.Slug == firstSlug {
			firstLibrarySlug = ch.LibrarySlug
		}
		if ch.Slug == lastSlug {
			lastLibrarySlug = ch.LibrarySlug
		}
	}

	// Get user-specific data
	userRole, lastReadChapterSlug, userCollections, mediaCollections, favCount, isFavorite, score, upvotes, downvotes, userVote, isHighlighted := getUserSpecificData(userName, slug, chapters, media)

	unit := "chapter"
	if media.Type == "novel" {
		unit = "volume"
	}

	if isHTMXRequest(c) {
		return handleView(c, views.MediaChaptersSection(*media, chapters, sortParam, lastReadChapterSlug, premiumDuration, userRole, isHighlighted, unit, userName, page, limit, chapterCount))
	}

	return handleView(c, views.Media(*media, chapters, firstSlug, lastSlug, firstLibrarySlug, lastLibrarySlug, chapterCount, userRole, lastReadChapterSlug, sortParam, page, limit, premiumDuration, userName, userCollections, mediaCollections, isHighlighted, favCount, isFavorite, score, upvotes, downvotes, userVote))
}

// checkMediaAccess checks if the user has access to the media based on library permissions
func checkMediaAccess(c fiber.Ctx, chapters []models.Chapter) error {
	hasAccess := len(chapters) == 0 // If no chapters, allow access (media might be empty or public)
	for _, chapter := range chapters {
		access, err := UserHasLibraryAccess(c, chapter.LibrarySlug)
		if err != nil {
			return SendInternalServerError(c, ErrInternalServerError, err)
		}
		if access {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		if isHTMXRequest(c) {
			triggerNotification(c, "Access denied: you don't have permission to view this media", "destructive")
			// Return 204 No Content to prevent navigation/swap but show notification
			return c.Status(fiber.StatusNoContent).SendString("")
		}
		return SendForbiddenError(c, ErrForbidden)
	}
	return nil
}

// getPaginationParams parses pagination and sort parameters from the request
func getPaginationParams(c fiber.Ctx) (page, limit int, sortParam string) {
	page = defaultPage
	limit = 25
	sortParam = "newest"
	if isHTMXRequest(c) && c.Method() == "POST" {
		// For HTMX POST requests, parse form data
		type PaginationForm struct {
			Page  int    `form:"page"`
			Limit int    `form:"limit"`
			Sort  string `form:"sort"`
		}
		var form PaginationForm
		if err := c.Bind().Body(&form); err == nil {
			if form.Page > 0 {
				page = form.Page
			}
			if form.Limit > 0 {
				limit = form.Limit
			}
			if form.Sort != "" {
				sortParam = form.Sort
			}
		}
	}
	return
}

// getChaptersData fetches paginated chapters for the media
func getChaptersData(slug string, offset, limit int, sortParam string, cfg models.AppConfig) ([]models.Chapter, int, error) {
	return models.GetChaptersByMediaSlugPaginated(slug, offset, limit, sortParam, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled)
}

// getFirstLastSlugs retrieves the first and last chapter slugs for the media
func getFirstLastSlugs(slug string) (firstSlug, lastSlug string) {
	var err error
	firstSlug, err = models.GetFirstChapterSlug(slug)
	if err != nil {
		log.Errorf("Error getting first chapter slug for %s: %v", slug, err)
		firstSlug = ""
	}
	lastSlug, err = models.GetLastChapterSlug(slug)
	if err != nil {
		log.Errorf("Error getting last chapter slug for %s: %v", slug, err)
		lastSlug = ""
	}
	return
}

// getUserSpecificData fetches all user-specific data for the media page
func getUserSpecificData(userName, slug string, chapters []models.Chapter, media *models.Media) (userRole string, lastReadChapterSlug string, userCollections, mediaCollections []models.Collection, favCount int, isFavorite bool, score, upvotes, downvotes, userVote int, isHighlighted bool) {
	if userName == "" {
		userRole = "anonymous"
		return
	}

	user, err := models.FindUserByUsername(userName)
	if err == nil && user != nil {
		userRole = user.Role
	}

	// Fetch read chapters and annotate the list
	readMap, err := models.GetReadChaptersForUser(userName, slug)
	if err == nil {
		for i := range chapters {
			chapters[i].Read = readMap[chapters[i].Slug]
		}
		media.ReadCount = len(readMap)
	}

	// Fetch the last read chapter for the resume button
	lastReadChapterSlug, _, err = models.GetLastReadChapter(userName, slug)
	// lastReadChapterSlug is already what we need - no conversion needed

	// Get accessible libraries
	var accessibleLibraries []string
	accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
	if err != nil {
		log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
		accessibleLibraries = []string{}
	}

	userCollections, err = models.GetCollectionsByUser(userName, accessibleLibraries)
	if err != nil {
		log.Errorf("Error getting user collections: %v", err)
		userCollections = []models.Collection{}
	}

	// Check which collections contain this media
	if len(userCollections) > 0 {
		collectionIDs := make([]int, len(userCollections))
		for i, c := range userCollections {
			collectionIDs[i] = c.ID
		}
		mediaInCollections, err := models.BatchCheckMediaInCollections(collectionIDs, slug)
		if err == nil {
			for _, collection := range userCollections {
				if mediaInCollections[collection.ID] {
					mediaCollections = append(mediaCollections, collection)
				}
			}
		}
	}

	// Fetch favorite data
	favCount, err = models.GetFavoritesCount(slug)
	if err != nil {
		log.Errorf("Error getting favorites count for %s: %v", slug, err)
		favCount = 0
	}
	isFav, _ := models.IsFavoriteForUser(userName, slug)
	isFavorite = isFav

	// Fetch vote data
	score, upvotes, downvotes, err = models.GetMediaVotes(slug)
	if err != nil {
		log.Errorf("Error getting votes for %s: %v", slug, err)
		score, upvotes, downvotes = 0, 0, 0
	}
	userV, _ := models.GetUserVoteForMedia(userName, slug)
	userVote = userV

	// Check if media is highlighted
	isHighlighted, err = models.IsMediaHighlighted(slug)
	if err != nil {
		log.Errorf("Error checking if media %s is highlighted: %v", slug, err)
		isHighlighted = false
	}

	return
}

// HandleMediaSearch returns search results for the quick-search panel.
func HandleMediaSearch(c fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		// Return empty content to hide results container
		return c.SendString("")
	}

	// Get accessible libraries for the current user
	accessibleLibraries, err := GetUserAccessibleLibraries(c)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	userName := GetUserContext(c)
	opts := models.SearchOptions{
		SearchFilter:        searchParam,
		Page:                defaultPage,
		PageSize:            searchPageSize,
		SortBy:              "name",
		SortOrder:           "desc",
		AccessibleLibraries: accessibleLibraries,
		ContentRatingLimit:  GetContentRatingLimit(userName),
	}
	media, _, err := models.SearchMediasWithOptions(opts)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	if len(media) == 0 {
		return handleView(c, views.NoResultsSearch())
	}

	return handleView(c, views.SearchMedias(media))
}

// HandleTags returns a JSON array of all known tags for client-side consumption
func HandleTags(c fiber.Ctx) error {
	tags, err := models.GetAllTags()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tags)
}

// HandleTagsFragment returns an HTMX-ready fragment with tag checkboxes
func HandleTagsFragment(c fiber.Ctx) error {
	tags, err := models.GetAllTags()
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	// Determine currently selected tags from the query (support repeated and comma-separated)
	var selectedTags []string
	if raw := string(c.Request().URI().QueryString()); raw != "" {
		if valsMap, err := url.ParseQuery(raw); err == nil {
			if vals, ok := valsMap["tags"]; ok {
				for _, v := range vals {
					for t := range strings.SplitSeq(v, ",") {
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
