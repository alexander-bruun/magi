package handlers

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
)

// QueryParams holds parsed query parameters for manga listings
type QueryParams struct {
	Page         int
	Sort         string
	Order        string
	Tags         []string
	TagMode      string
	LibrarySlug  string
	SearchFilter string
}

// ParseQueryParams extracts and normalizes query parameters from the request
func ParseQueryParams(c *fiber.Ctx) QueryParams {
	params := QueryParams{
		Page:    getPageNumber(c.Query("page")),
		TagMode: strings.ToLower(c.Query("tag_mode")),
	}

	// Parse sorting parameters - handle duplicates from HTMX includes
	if raw := string(c.Request().URI().QueryString()); raw != "" {
		if valsMap, err := url.ParseQuery(raw); err == nil {
			if vals, ok := valsMap["sort"]; ok && len(vals) > 0 {
				params.Sort = vals[0]
			}
			if vals, ok := valsMap["order"]; ok && len(vals) > 0 {
				params.Order = vals[0]
			}
			// Parse tags from repeated params
			if vals, ok := valsMap["tags"]; ok {
				for _, v := range vals {
					for _, t := range strings.Split(v, ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							params.Tags = append(params.Tags, t)
						}
					}
				}
			}
		}
	}

	// Fallback to simple query params
	if params.Sort == "" {
		params.Sort = c.Query("sort")
	}
	if params.Order == "" {
		params.Order = c.Query("order")
	}

	// Fallback for comma-separated tags
	if len(params.Tags) == 0 {
		if raw := c.Query("tags"); raw != "" {
			for _, t := range strings.Split(raw, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					params.Tags = append(params.Tags, t)
				}
			}
		}
	}

	// Normalize defaults
	if params.Sort == "" && params.Order == "" {
		params.Sort, params.Order = models.MangaSortConfig.DefaultKey, models.MangaSortConfig.DefaultOrder
	} else {
		params.Sort, params.Order = models.MangaSortConfig.NormalizeSort(params.Sort, params.Order)
	}

	if params.TagMode != "any" {
		params.TagMode = "all"
	}

	params.LibrarySlug = c.Query("library")
	params.SearchFilter = c.Query("search")

	return params
}

// getPageNumber extracts and validates the page number
func getPageNumber(pageStr string) int {
	if pageStr == "" {
		return defaultPage
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return defaultPage
	}
	return page
}

// CalculateTotalPages computes total pages from count and page size
func CalculateTotalPages(count int64, pageSize int) int {
	totalPages := int((count + int64(pageSize) - 1) / int64(pageSize))
	if totalPages == 0 {
		return 1
	}
	return totalPages
}

// GetUserContext extracts username from fiber context locals
func GetUserContext(c *fiber.Ctx) string {
	if userName, ok := c.Locals("user_name").(string); ok && userName != "" {
		return userName
	}
	return ""
}

// IsHTMXRequest checks if the request is from HTMX
func IsHTMXRequest(c *fiber.Ctx) bool {
	return c.Get("HX-Request") == "true"
}

// GetHTMXTarget returns the HTMX target ID
func GetHTMXTarget(c *fiber.Ctx) string {
	return c.Get("HX-Target")
}
