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
func GetMediaListData(params models.QueryParams, userName string) (*MediaListData, error) {
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
		AccessibleLibraries: accessibleLibraries,
		ContentRatingLimit:  cfg.ContentRatingLimit,
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
	if IsHTMXRequest(c) {
		target := GetHTMXTarget(c)
		if target == "media-listing" {
			return HandleView(c, templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
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
		} else if target == "media-listing-results" {
			path := "/series"
			targetID := "media-listing-results"
			emptyMessage := "No media have been indexed yet."
			return HandleView(c, views.MediaListingFragment(
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

	return HandleView(c, views.MediasWithTypes(data.Media, params.Page, data.TotalPages, params.Sort, params.Order, params.Tags, params.TagMode, data.AllTags, params.Types, data.AllTypes, params.SearchFilter))
}

// HandleMedia renders a media detail page including chapters and per-user state.
func HandleMedia(c *fiber.Ctx) error {
	slug := c.Params("media")
	media, err := models.GetMedia(slug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if media == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, media.LibrarySlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if !hasAccess {
		if IsHTMXRequest(c) {
			triggerNotification(c, "Access denied: you don't have permission to view this media", "destructive")
			// Return 204 No Content to prevent navigation/swap but show notification
			return c.Status(fiber.StatusNoContent).SendString("")
		}
		return sendForbiddenError(c, ErrForbidden)
	}

	cfg, err := models.GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config: %v", err)
	}

	chapters, err := models.GetChaptersByMediaSlug(slug, 1000, cfg.MaxPremiumChapters, cfg.PremiumEarlyAccessDuration, cfg.PremiumCooldownScalingEnabled)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Precompute first/last chapter slugs before reversing
	firstSlug, lastSlug := models.GetFirstAndLastChapterSlugs(chapters)

	reverse := c.Query("reverse") == "true"
	if reverse {
		slices.Reverse(chapters)
	}

	// Get user role for conditional rendering
	userRole := ""
	userName := GetUserContext(c)
	lastReadChapterSlug := ""
	var userReview *models.Review
	var userCollections []models.Collection
	var mediaCollections []models.Collection
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
		lastReadChapter, err := models.GetLastReadChapter(userName, slug)
		if err == nil {
			lastReadChapterSlug = lastReadChapter
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

	if IsHTMXRequest(c) && c.Query("reverse") != "" {
		return HandleView(c, views.MediaChaptersSection(*media, chapters, reverse, lastReadChapterSlug, cfg.PremiumEarlyAccessDuration, userRole, isHighlighted))
	}

	if IsHTMXRequest(c) {
		return HandleView(c, views.Media(*media, chapters, firstSlug, lastSlug, len(chapters), userRole, lastReadChapterSlug, reverse, cfg.PremiumEarlyAccessDuration, reviews, userReview, userName, userCollections, mediaCollections, isHighlighted))
	}

	return HandleView(c, views.Media(*media, chapters, firstSlug, lastSlug, len(chapters), userRole, lastReadChapterSlug, reverse, cfg.PremiumEarlyAccessDuration, reviews, userReview, userName, userCollections, mediaCollections, isHighlighted))
} // HandleMediaSearch returns search results for the quick-search panel.
func HandleMediaSearch(c *fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		return HandleView(c, views.OneDoesNotSimplySearch())
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
	}
	media, _, err := models.SearchMediasWithOptions(opts)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	if len(media) == 0 {
		return HandleView(c, views.NoResultsSearch())
	}

	return HandleView(c, views.SearchMedias(media))
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
