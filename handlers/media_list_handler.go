package handlers

import (
	"context"
	"fmt"
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
		Filter:              params.SearchFilter,
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
		return handleError(c, err)
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
		return handleError(c, err)
	}
	if media == nil {
		return handleErrorWithStatus(c, fmt.Errorf("media not found or access restricted based on content rating settings"), fiber.StatusNotFound)
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, media.LibrarySlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		if IsHTMXRequest(c) {
			c.Set("HX-Trigger", `{"showNotification": {"message": "Access denied: you don't have permission to view this media", "status": "destructive"}}`)
			c.Set("HX-Redirect", "/")
			return c.SendString("")
		}
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this media"), fiber.StatusForbidden)
	}

	chapters, err := models.GetChapters(slug)
	if err != nil {
		return handleError(c, err)
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
	}
	
	cfg, err := models.GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config: %v", err)
	}
	
	if IsHTMXRequest(c) && c.Query("reverse") != "" {
		return HandleView(c, views.MediaChaptersSection(*media, chapters, reverse, lastReadChapterSlug, cfg.PremiumEarlyAccessDuration, userRole))
	}
	
	if IsHTMXRequest(c) {
		return HandleView(c, views.Media(*media, chapters, firstSlug, lastSlug, len(chapters), userRole, lastReadChapterSlug, reverse, cfg.PremiumEarlyAccessDuration))
	}
	
	return HandleView(c, views.Media(*media, chapters, firstSlug, lastSlug, len(chapters), userRole, lastReadChapterSlug, reverse, cfg.PremiumEarlyAccessDuration))
}// HandleMediaSearch returns search results for the quick-search panel.
func HandleMediaSearch(c *fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		return HandleView(c, views.OneDoesNotSimplySearch())
	}

	// Get accessible libraries for the current user
	accessibleLibraries, err := GetUserAccessibleLibraries(c)
	if err != nil {
		return handleError(c, err)
	}

	opts := models.SearchOptions{
		Filter:              searchParam,
		Page:                defaultPage,
		PageSize:            searchPageSize,
		SortBy:              "name",
		SortOrder:           "desc",
		AccessibleLibraries: accessibleLibraries,
	}
	media, _, err := models.SearchMediasWithOptions(opts)
	if err != nil {
		return handleError(c, err)
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