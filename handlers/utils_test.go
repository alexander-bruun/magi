package handlers

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	
	"github.com/alexander-bruun/magi/models"
)

func TestGetPageNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 1},       // empty string -> default page
		{"1", 1},      // valid page
		{"5", 5},      // valid page
		{"0", 1},      // invalid page (< 1) -> default
		{"-1", 1},     // invalid page (< 1) -> default
		{"abc", 1},    // invalid number -> default
		{"1.5", 1},    // invalid number -> default
	}

	for _, test := range tests {
		result := getPageNumber(test.input)
		assert.Equal(t, test.expected, result, "getPageNumber(%s)", test.input)
	}
}

func TestCalculateTotalPages(t *testing.T) {
	tests := []struct {
		count    int64
		pageSize int
		expected int
	}{
		{0, 10, 1},     // empty -> 1 page
		{1, 10, 1},     // 1 item -> 1 page
		{10, 10, 1},    // exactly page size -> 1 page
		{11, 10, 2},    // 11 items -> 2 pages
		{20, 10, 2},    // exactly 2 pages
		{21, 10, 3},    // 21 items -> 3 pages
		{100, 25, 4},   // 100/25 = 4
		{1, 1, 1},      // page size 1
	}

	for _, test := range tests {
		result := CalculateTotalPages(test.count, test.pageSize)
		assert.Equal(t, test.expected, result, "CalculateTotalPages(%d, %d)", test.count, test.pageSize)
	}
}

func TestIsHTMXRequest(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test HTMX request
	c.Request().Header.Set("HX-Request", "true")
	result := IsHTMXRequest(c)
	assert.True(t, result)

	// Test non-HTMX request
	c.Request().Header.Set("HX-Request", "false")
	result = IsHTMXRequest(c)
	assert.False(t, result)

	// Test missing header
	c.Request().Header.Del("HX-Request")
	result = IsHTMXRequest(c)
	assert.False(t, result)

	app.ReleaseCtx(c)
}

func TestGetHTMXTarget(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test with target
	c.Request().Header.Set("HX-Target", "my-target")
	result := GetHTMXTarget(c)
	assert.Equal(t, "my-target", result)

	// Test without target
	c.Request().Header.Del("HX-Target")
	result = GetHTMXTarget(c)
	assert.Equal(t, "", result)

	app.ReleaseCtx(c)
}

func TestParseQueryParams(t *testing.T) {
	app := fiber.New()

	tests := []struct {
		name     string
		url      string
		expected QueryParams
	}{
		{
			name: "default values",
			url:  "/",
			expected: QueryParams{
				Page:    1,
				TagMode: "all",
				Sort:    models.MediaSortConfig.DefaultKey,
				Order:   models.MediaSortConfig.DefaultOrder,
			},
		},
		{
			name: "with page",
			url:  "/?page=3",
			expected: QueryParams{
				Page:    3,
				TagMode: "all",
				Sort:    models.MediaSortConfig.DefaultKey,
				Order:   models.MediaSortConfig.DefaultOrder,
			},
		},
		{
			name: "with sort and order",
			url:  "/?sort=title&order=asc",
			expected: QueryParams{
				Page:    1,
				TagMode: "all",
				Sort:    "name", // "title" gets normalized to "name"
				Order:   "asc",
			},
		},
		{
			name: "with tags",
			url:  "/?tags=action,drama",
			expected: QueryParams{
				Page:    1,
				TagMode: "all",
				Sort:    models.MediaSortConfig.DefaultKey,
				Order:   models.MediaSortConfig.DefaultOrder,
				Tags:   []string{"action", "drama"},
			},
		},
		{
			name: "with types",
			url:  "/?types=manga,manhwa",
			expected: QueryParams{
				Page:    1,
				TagMode: "all",
				Sort:    models.MediaSortConfig.DefaultKey,
				Order:   models.MediaSortConfig.DefaultOrder,
				Types:  []string{"manga", "manhwa"},
			},
		},
		{
			name: "with tag_mode any",
			url:  "/?tag_mode=any",
			expected: QueryParams{
				Page:    1,
				TagMode: "any",
				Sort:    models.MediaSortConfig.DefaultKey,
				Order:   models.MediaSortConfig.DefaultOrder,
			},
		},
		{
			name: "with library and search",
			url:  "/?library=my-lib&search=test query",
			expected: QueryParams{
				Page:         1,
				TagMode:      "all",
				Sort:         models.MediaSortConfig.DefaultKey,
				Order:        models.MediaSortConfig.DefaultOrder,
				LibrarySlug: "my-lib",
				SearchFilter: "test query",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &fasthttp.Request{}
			req.SetRequestURI(test.url)
			req.Header.SetMethod("GET")

			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			ctx.Request().SetRequestURI(test.url)

			result := ParseQueryParams(ctx)
			
			// Check the expected fields
			assert.Equal(t, test.expected.Page, result.Page)
			assert.Equal(t, test.expected.TagMode, result.TagMode)
			assert.Equal(t, test.expected.Sort, result.Sort)
			assert.Equal(t, test.expected.Order, result.Order)
			assert.Equal(t, test.expected.Tags, result.Tags)
			assert.Equal(t, test.expected.Types, result.Types)
			assert.Equal(t, test.expected.LibrarySlug, result.LibrarySlug)
			assert.Equal(t, test.expected.SearchFilter, result.SearchFilter)

			app.ReleaseCtx(ctx)
		})
	}
}

func TestGetUserContext(t *testing.T) {
	app := fiber.New()

	// Test with valid username
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Locals("user_name", "testuser")
	result := GetUserContext(c)
	assert.Equal(t, "testuser", result)
	app.ReleaseCtx(c)

	// Test with empty username
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Locals("user_name", "")
	result = GetUserContext(c)
	assert.Equal(t, "", result)
	app.ReleaseCtx(c)

	// Test with non-string value
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Locals("user_name", 123)
	result = GetUserContext(c)
	assert.Equal(t, "", result)
	app.ReleaseCtx(c)

	// Test with missing key
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	result = GetUserContext(c)
	assert.Equal(t, "", result)
	app.ReleaseCtx(c)
}

func TestGetPageNumberEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"1000000", 1000000},  // very large number
		{"00001", 1},          // leading zeros
		{"+1", 1},             // should fail, return default
		{"1 ", 1},             // space should fail, return default
		{" 1", 1},             // leading space should fail, return default
	}

	for _, test := range tests {
		result := getPageNumber(test.input)
		assert.Equal(t, test.expected, result, "getPageNumber(%s)", test.input)
	}
}

func TestCalculateTotalPagesLargeNumbers(t *testing.T) {
	tests := []struct {
		count    int64
		pageSize int
		expected int
	}{
		{1000000, 10, 100000},     // large count
		{1, 1000, 1},              // large page size
		{999999, 10, 100000},      // near boundary
		{10000000, 100, 100000},   // very large
	}

	for _, test := range tests {
		result := CalculateTotalPages(test.count, test.pageSize)
		assert.Equal(t, test.expected, result, "CalculateTotalPages(%d, %d)", test.count, test.pageSize)
	}
}

func TestIsHTMXRequestVariations(t *testing.T) {
	app := fiber.New()
	tests := []struct {
		name        string
		headerVal   string
		expected    bool
	}{
		{"true lowercase", "true", true},
		{"false lowercase", "false", false},
		{"TRUE uppercase", "TRUE", false},  // must be exact match "true"
		{"empty string", "", false},
		{"1 instead of true", "1", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().Header.Set("HX-Request", test.headerVal)
			result := IsHTMXRequest(c)
			assert.Equal(t, test.expected, result)
			app.ReleaseCtx(c)
		})
	}
}

func TestParseQueryParamsEdgeCases(t *testing.T) {
	app := fiber.New()
	tests := []struct {
		name     string
		url      string
		checkFn  func(t *testing.T, result QueryParams)
	}{
		{
			name: "invalid page number",
			url:  "/?page=invalid",
			checkFn: func(t *testing.T, result QueryParams) {
				assert.Equal(t, 1, result.Page)
			},
		},
		{
			name: "negative page number",
			url:  "/?page=-5",
			checkFn: func(t *testing.T, result QueryParams) {
				assert.Equal(t, 1, result.Page)
			},
		},
		{
			name: "empty tags",
			url:  "/?tags=",
			checkFn: func(t *testing.T, result QueryParams) {
				assert.Empty(t, result.Tags)
			},
		},
		{
			name: "tags with whitespace",
			url:  "/?tags=action , drama , comedy",
			checkFn: func(t *testing.T, result QueryParams) {
				assert.Contains(t, result.Tags, "action")
				assert.Contains(t, result.Tags, "drama")
				assert.Contains(t, result.Tags, "comedy")
			},
		},
		{
			name: "tag_mode invalid",
			url:  "/?tag_mode=invalid",
			checkFn: func(t *testing.T, result QueryParams) {
				assert.Equal(t, "all", result.TagMode) // defaults to "all"
			},
		},
		{
			name: "search with special chars",
			url:  "/?search=test%20query%26more",
			checkFn: func(t *testing.T, result QueryParams) {
				assert.NotEmpty(t, result.SearchFilter)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			ctx.Request().SetRequestURI(test.url)
			result := ParseQueryParams(ctx)
			test.checkFn(t, result)
			app.ReleaseCtx(ctx)
		})
	}
}