package handlers

import (
	"context"
	"io"
	"net/url"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
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
	contentRatingLimit := cfg.ContentRatingLimit
	accessibleLibs := accessibleLibraries
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil && user.Role == "admin" {
			contentRatingLimit = 3      // Admins can see all content
			accessibleLibs = []string{} // Admins can access all libraries without filter
		}
	}

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
func HandleMedias(c *fiber.Ctx) error {
	params := ParseQueryParams(c)
	userName := GetUserContext(c)

	data, err := GetMediaListData(params, userName)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
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
				err = views.GenericMediaListingWithTypes("/series", "media-listing", true, data.Media, params.Page, data.TotalPages, params.Sort, params.Order, "No media have been indexed yet.", params.Tags, params.TagMode, data.AllTags, params.Types, data.AllTypes, params.SearchFilter).Render(ctx, w)
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

	return handleView(c, views.MediasWithTypes(data.Media, params.Page, data.TotalPages, params.Sort, params.Order, params.Tags, params.TagMode, data.AllTags, params.Types, data.AllTypes, params.SearchFilter))
}

// HandleMedia renders a media detail page including chapters and per-user state.
func HandleMedia(c *fiber.Ctx) error {
	slug := c.Params("media")
	userName := GetUserContext(c)

	// Get content rating limit based on user
	cfg, err := models.GetAppConfig()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	contentRatingLimit := cfg.ContentRatingLimit
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil && user.Role == "admin" {
			contentRatingLimit = 3 // Admins can see all content
		}
	}

	media, err := models.GetMediaWithContentLimit(slug, contentRatingLimit)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if media == nil {
		log.Infof("HandleMedia: media not found for slug=%s", slug)
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Check library access permission
	chapters, err := models.GetChapters(slug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	hasAccess := len(chapters) == 0 // If no chapters, allow access (media might be empty or public)
	for _, chapter := range chapters {
		access, err := UserHasLibraryAccess(c, chapter.LibrarySlug)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
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
		return sendForbiddenError(c, ErrForbidden)
	}

	cfg, err = models.GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config: %v", err)
	}

	premiumDuration := cfg.PremiumEarlyAccessDuration

	chapters, err = models.GetChaptersByMediaSlug(slug, 1000, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Precompute first/last chapter IDs before reversing
	firstID, lastID := "", ""
	if len(chapters) > 0 {
		firstID = chapters[0].ID
		lastID = chapters[len(chapters)-1].ID
	}

	// Find library slugs for first and last chapters (keeping for compatibility, but not used in new URLs)
	firstLibrarySlug := ""
	lastLibrarySlug := ""
	for _, ch := range chapters {
		if ch.ID == firstID {
			firstLibrarySlug = ch.LibrarySlug
		}
		if ch.ID == lastID {
			lastLibrarySlug = ch.LibrarySlug
		}
	}

	reverse := c.Query("reverse") == "true"
	if reverse {
		slices.Reverse(chapters)
	}

	// Get user role for conditional rendering
	userRole := ""
	userName = GetUserContext(c)
	lastReadChapterID := ""
	var userReview *models.Review
	var userCollections []models.Collection
	var mediaCollections []models.Collection
	favCount := 0
	isFavorite := false
	score := 0
	upvotes := 0
	downvotes := 0
	userVote := 0
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
		// Fetch the last read chapter for the resume button
		lastReadChapterSlug, _, err := models.GetLastReadChapter(userName, slug)
		if err == nil && lastReadChapterSlug != "" {
			// Find the chapter ID
			for _, ch := range chapters {
				if ch.Slug == lastReadChapterSlug {
					lastReadChapterID = ch.ID
					break
				}
			}
		}
		// Fetch user's review if exists
		userReview, err = models.GetReviewByUserAndMedia(userName, slug)
		if err != nil {
			log.Errorf("Error getting user review for %s: %v", slug, err)
		}
		// Fetch user's collections
		var accessibleLibraries []string
		if userName != "" {
			accessibleLibraries, err = models.GetAccessibleLibrariesForUser(userName)
			if err != nil {
				log.Errorf("Failed to get accessible libraries for user %s: %v", userName, err)
				accessibleLibraries = []string{} // Empty if error
			}
		} else {
			accessibleLibraries, err = models.GetAccessibleLibrariesForAnonymous()
			if err != nil {
				log.Errorf("Failed to get accessible libraries for anonymous: %v", err)
				accessibleLibraries = []string{} // Empty if error
			}
		}
		userCollections, err = models.GetCollectionsByUser(userName, accessibleLibraries)
		if err != nil {
			log.Errorf("Error getting user collections: %v", err)
			userCollections = []models.Collection{}
		}
		// Check which collections contain this media (batch operation)
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
	}

	// Fetch favorite data
	favCount, err = models.GetFavoritesCount(slug)
	if err != nil {
		log.Errorf("Error getting favorites count for %s: %v", slug, err)
		favCount = 0
	}
	if userName != "" {
		isFav, _ := models.IsFavoriteForUser(userName, slug)
		isFavorite = isFav
	}

	// Fetch vote data
	score, upvotes, downvotes, err = models.GetMediaVotes(slug)
	if err != nil {
		log.Errorf("Error getting votes for %s: %v", slug, err)
		score, upvotes, downvotes = 0, 0, 0
	}
	if userName != "" {
		userV, _ := models.GetUserVoteForMedia(userName, slug)
		userVote = userV
	}

	// Fetch all reviews for the media
	reviews, err := models.GetReviewsByMedia(slug)
	if err != nil {
		log.Errorf("Error getting reviews for %s: %v", slug, err)
		reviews = []models.Review{} // Initialize empty slice on error
	}

	// Check if media is highlighted
	isHighlighted, err := models.IsMediaHighlighted(slug)
	if err != nil {
		log.Errorf("Error checking if media %s is highlighted: %v", slug, err)
		isHighlighted = false
	}

	if isHTMXRequest(c) {
		return handleView(c, views.Media(*media, chapters, firstID, lastID, firstLibrarySlug, lastLibrarySlug, len(chapters), userRole, lastReadChapterID, reverse, premiumDuration, reviews, userReview, userName, userCollections, mediaCollections, isHighlighted, favCount, isFavorite, score, upvotes, downvotes, userVote))
	}

	return handleView(c, views.Media(*media, chapters, firstID, lastID, firstLibrarySlug, lastLibrarySlug, len(chapters), userRole, lastReadChapterID, reverse, premiumDuration, reviews, userReview, userName, userCollections, mediaCollections, isHighlighted, favCount, isFavorite, score, upvotes, downvotes, userVote))
} // HandleMediaSearch returns search results for the quick-search panel.
func HandleMediaSearch(c *fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		return handleView(c, views.OneDoesNotSimplySearch())
	}

	// Get app config for content rating limit
	cfg, err := models.GetAppConfig()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Get accessible libraries for the current user
	accessibleLibraries, err := GetUserAccessibleLibraries(c)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	opts := models.SearchOptions{
		SearchFilter:        searchParam,
		Page:                defaultPage,
		PageSize:            searchPageSize,
		SortBy:              "name",
		SortOrder:           "desc",
		AccessibleLibraries: accessibleLibraries,
		ContentRatingLimit:  cfg.ContentRatingLimit,
	}
	media, _, err := models.SearchMediasWithOptions(opts)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	if len(media) == 0 {
		return handleView(c, views.NoResultsSearch())
	}

	return handleView(c, views.SearchMedias(media))
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
		return sendInternalServerError(c, ErrInternalServerError, err)
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
