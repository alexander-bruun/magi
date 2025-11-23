package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

const (
	defaultLightNovelPage     = 1
	defaultLightNovelPageSize = 16
	searchLightNovelPageSize  = 10
)

// HandleLightNovels handles the light novels listing page
func HandleLightNovels(c *fiber.Ctx) error {
	params := ParseQueryParams(c)

	cfg, err := models.GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	// Get accessible libraries for the current user
	accessibleLibraries, err := GetUserAccessibleLibraries(c)
	if err != nil {
		return handleError(c, err)
	}

	log.Debugf("Accessible libraries: %v", accessibleLibraries)

	// Search light novels using options
	opts := models.LightNovelSearchOptions{
		Filter:              params.SearchFilter,
		Page:                params.Page,
		PageSize:            defaultLightNovelPageSize,
		SortBy:              params.Sort,
		SortOrder:           params.Order,
		LibrarySlug:         params.LibrarySlug,
		AccessibleLibraries: accessibleLibraries,
	}
	lightNovels, count, err := models.SearchLightNovelsWithOptions(opts, cfg.ContentRatingLimit)

	if err != nil {
		log.Errorf("Failed to get light novels: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	log.Debugf("Found %d light novels", count)

	totalPages := CalculateTotalPages(count, defaultLightNovelPageSize)

	// If HTMX request targeting the listing container, render just the generic listing
	if IsHTMXRequest(c) && GetHTMXTarget(c) == "light-novel-listing" {
		return HandleView(c, views.GenericLightNovelListing("/light-novels", "light-novel-listing", lightNovels, params.Page, totalPages, params.Sort, params.Order, "No light novels have been indexed yet.", params.SearchFilter))
	}

	return HandleView(c, views.LightNovels(lightNovels, params.Page, totalPages, params.Sort, params.Order, params.SearchFilter))
}

// HandleLightNovel handles individual light novel page
func HandleLightNovel(c *fiber.Ctx) error {
	slug := c.Params("light_novel")

	lightNovel, err := models.GetLightNovel(slug)
	if err != nil {
		log.Errorf("Failed to get light novel %s: %v", slug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if lightNovel == nil {
		return c.Status(fiber.StatusNotFound).SendString("Light novel not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, lightNovel.LibrarySlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this light novel"), fiber.StatusForbidden)
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

	if IsHTMXRequest(c) && c.Query("reverse") != "" {
		return HandleView(c, views.LightNovelChaptersSection(*lightNovel, chapters, reverse, lastReadChapterSlug))
	}

	return HandleView(c, views.LightNovel(*lightNovel, chapters, firstSlug, lastSlug, len(chapters), userRole, lastReadChapterSlug, reverse))
}

// HandleLightNovelSearch handles light novel search
func HandleLightNovelSearch(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Redirect("/light-novels")
	}

	page := c.QueryInt("page", defaultLightNovelPage)
	pageSize := c.QueryInt("page_size", searchLightNovelPageSize)
	librarySlug := c.Query("library")

	cfg, err := models.GetAppConfig()
	if err != nil {
		log.Errorf("Failed to get app config: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	lightNovels, err := models.SearchLightNovels(query, pageSize, (page-1)*pageSize, librarySlug, cfg.ContentRatingLimit)
	if err != nil {
		log.Errorf("Failed to search light novels: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	totalLightNovels := len(lightNovels) // Approximate for search
	totalPages := (totalLightNovels + pageSize - 1) / pageSize

	libraries, err := models.GetLibraries()
	if err != nil {
		log.Errorf("Failed to get libraries: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	return c.JSON(fiber.Map{
		"light_novels": lightNovels,
		"query":        query,
		"page":         page,
		"total_pages":  totalPages,
		"libraries":    libraries,
	})
}

// HandleLightNovelFavorite handles toggling a favorite for the logged-in user via HTMX
func HandleLightNovelFavorite(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	value := c.FormValue("value")
	if value == "1" {
		if err := models.SetLightNovelFavorite(userName, lightNovelSlug); err != nil {
			return handleError(c, err)
		}
	} else {
		if err := models.RemoveLightNovelFavorite(userName, lightNovelSlug); err != nil {
			return handleError(c, err)
		}
	}

	// Return the favorite fragment so HTMX will swap the icon in-place.
	return HandleLightNovelFavoriteFragment(c)
}

// HandleLightNovelFavoriteFragment returns the favorite UI fragment for a light novel
func HandleLightNovelFavoriteFragment(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	userName := GetUserContext(c)

	favCount, err := models.GetLightNovelFavoritesCount(lightNovelSlug)
	if err != nil {
		return handleError(c, err)
	}

	isFavorite := false
	if userName != "" {
		isFavorite, err = models.IsLightNovelFavoriteForUser(userName, lightNovelSlug)
		if err != nil {
			return handleError(c, err)
		}
	}

	return HandleView(c, views.LightNovelFavoriteFragment(lightNovelSlug, favCount, isFavorite))
}

// HandleLightNovelVote handles voting on light novels
func HandleLightNovelVote(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	valueStr := c.FormValue("value")
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return handleError(c, fmt.Errorf("invalid vote value"))
	}

	if value == 0 {
		if err := models.RemoveLightNovelVote(userName, lightNovelSlug); err != nil {
			return handleError(c, err)
		}
	} else {
		if err := models.SetLightNovelVote(userName, lightNovelSlug, value); err != nil {
			return handleError(c, err)
		}
	}

	// Return the vote fragment so HTMX will swap the UI in-place.
	return HandleLightNovelVoteFragment(c)
}

// HandleLightNovelVoteFragment handles HTMX request for vote fragment
func HandleLightNovelVoteFragment(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	userName := GetUserContext(c)

	score, upvotes, downvotes, err := models.GetLightNovelVotes(lightNovelSlug)
	if err != nil {
		return handleError(c, err)
	}

	userVote := 0
	if userName != "" {
		userVote, err = models.GetUserVoteForLightNovel(userName, lightNovelSlug)
		if err != nil {
			return handleError(c, err)
		}
	}

	return HandleView(c, views.LightNovelVoteFragment(lightNovelSlug, score, upvotes, downvotes, userVote))
}

// HandleLightNovelMarkRead marks a light novel chapter as read for the logged-in user via HTMX
func HandleLightNovelMarkRead(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	chapterSlug := c.Params("chapter")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}
	if err := models.MarkChapterRead(userName, lightNovelSlug, chapterSlug); err != nil {
		return handleError(c, err)
	}
	// No content to return since hx-swap="none"
	return c.SendString("")
}

// EPUBContent represents the extracted content from an EPUB file
type EPUBContent struct {
	Title       string
	Chapters    []EPUBChapter
	CSS         []string
	Assets      map[string][]byte
	Metadata    map[string]string
}

// EPUBChapter represents a single chapter/page in the EPUB
type EPUBChapter struct {
	ID      string
	Title   string
	Content string
	Href    string
}

// HandleReadLightNovelChapter handles reading a light novel chapter inline
func HandleReadLightNovelChapter(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(lightNovelSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", lightNovelSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	// Get light novel info for breadcrumbs
	lightNovel, err := models.GetLightNovel(lightNovelSlug)
	if err != nil {
		log.Errorf("Failed to get light novel %s: %v", lightNovelSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	// Mark chapter as read if user is logged in
	userName := GetUserContext(c)
	if userName != "" {
		err := models.MarkChapterRead(userName, lightNovelSlug, chapterSlug)
		if err != nil {
			log.Warnf("Failed to mark chapter as read: %v", err)
		}
	}

	// Return a simple HTML page that loads content dynamically
	c.Set("Content-Type", "text/html")
	return c.SendString(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - %s</title>
    <style>
        body { margin: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; overflow: hidden; background: #f8f9fa; }
        .header { padding: 20px; background: white; border-bottom: 1px solid #e2e8f0; }
        .header h1 { margin: 0; font-size: 1.5em; }
        .header .actions { margin-top: 10px; }
        .header .actions a { margin-right: 10px; padding: 8px 16px; text-decoration: none; border: 1px solid #e2e8f0; border-radius: 4px; }
        .container { display: flex; height: calc(100vh - 100px); }
        .toc { 
            width: 320px; 
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); 
            padding: 20px 15px; 
            overflow-y: auto; 
            overflow-x: hidden; 
            position: fixed; 
            left: 0; 
            top: 100px; 
            height: calc(100vh - 100px); 
            border-right: 2px solid #5a67d8; 
            box-sizing: border-box; 
            box-shadow: 2px 0 10px rgba(0,0,0,0.1);
            color: #fff;
        }
        .toc h2 { 
            margin: 0 0 20px 0; 
            font-size: 1.5em; 
            font-weight: 600; 
            text-align: center; 
            color: #fff;
            text-shadow: 0 1px 2px rgba(0,0,0,0.3);
        }
        .toc nav { margin: 0; padding: 0; }
        .toc ol, .toc ul { 
            list-style: none; 
            margin: 0; 
            padding: 0; 
        }
        .toc li { 
            margin: 8px 0; 
            border-radius: 8px; 
            overflow: hidden;
            transition: all 0.3s ease;
        }
        .toc a { 
            text-decoration: none; 
            color: #e2e8f0; 
            display: block; 
            padding: 12px 15px; 
            word-break: break-all; 
            white-space: normal; 
            box-sizing: border-box; 
            max-width: 100%%; 
            text-indent: 0; 
            border-radius: 8px;
            transition: all 0.3s ease;
            font-weight: 400;
            font-size: 0.95em;
            line-height: 1.4;
        }
        .toc a:hover { 
            background: rgba(255,255,255,0.1); 
            color: #fff; 
            transform: translateX(5px);
        }
        .toc a.active { 
            background: #4299e1; 
            color: white; 
            box-shadow: 0 2px 8px rgba(66, 153, 225, 0.4);
            transform: translateX(5px);
        }
        .content { 
            margin-left: 320px; 
            padding: 30px; 
            overflow-y: auto; 
            height: calc(100vh - 100px); 
            background: #fff;
            line-height: 1.6;
            color: #2d3748;
        }
        .content img {
            max-width: 100%%;
            height: auto;
            display: block;
            margin: 10px auto;
        }
        .toc::-webkit-scrollbar {
            width: 8px;
        }
        .toc::-webkit-scrollbar-track {
            background: rgba(255,255,255,0.1);
        }
        .toc::-webkit-scrollbar-thumb {
            background: rgba(255,255,255,0.3);
            border-radius: 4px;
        }
        .toc::-webkit-scrollbar-thumb:hover {
            background: rgba(255,255,255,0.5);
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>%s</h1>
        <div class="actions">
            <a href="/light-novels/%s">Back to Chapters</a>
        </div>
    </div>
    <div class="container">
        <div id="toc" class="toc">
            <h2>Table of Contents</h2>
        </div>
        <div id="content" class="content">
            <p>Loading book...</p>
        </div>
    </div>

    <script>
        const lightNovelSlug = '%s';
        const chapterSlug = '%s';

        // Fetch TOC
        fetch('/light-novels/' + lightNovelSlug + '/chapters/' + chapterSlug + '/toc')
            .then(response => response.text())
            .then(html => {
                document.getElementById('toc').innerHTML += html;
            })
            .catch(error => {
                document.getElementById('toc').innerHTML += '<p>Error loading TOC</p>';
            });

        // Fetch book content
        fetch('/light-novels/' + lightNovelSlug + '/chapters/' + chapterSlug + '/content')
            .then(response => response.text())
            .then(html => {
                document.getElementById('content').innerHTML = html;
                // After loading content, set up scroll tracking
                setupTOC();
            })
            .catch(error => {
                document.getElementById('content').innerHTML = '<p>Error loading book: ' + error + '</p>';
            });

        function setupTOC() {
            const tocLinks = document.querySelectorAll('.toc a');
            const chapters = document.querySelectorAll('[id^="chapter-"]');
            const content = document.getElementById('content');

            // Filter chapters to only those in the TOC
            const tocChapterIds = new Set();
            tocLinks.forEach(link => {
                tocChapterIds.add(link.getAttribute('href').substring(1));
            });
            const tocChapters = Array.from(chapters).filter(chapter => tocChapterIds.has(chapter.id));

            // Add click handlers for smooth scrolling
            tocLinks.forEach(link => {
                link.addEventListener('click', e => {
                    e.preventDefault();
                    const targetId = link.getAttribute('href').substring(1);
                    const target = document.getElementById(targetId);
                    if (target) {
                        target.scrollIntoView({ behavior: 'smooth', block: 'start' });
                    }
                });
            });

            let previousCurrent = '';
            function updateActive() {
                const scrollY = content.scrollTop;
                const paddingTop = parseInt(getComputedStyle(content).paddingTop) || 0;
                const effectiveScrollY = scrollY + paddingTop;
                let current = '';

                tocChapters.forEach(chapter => {
                    const top = chapter.offsetTop;
                    const bottom = top + chapter.offsetHeight;
                    if (effectiveScrollY >= top) {
                        current = chapter.id;
                    }
                });

                if (current === '') {
                    current = previousCurrent;
                } else {
                    previousCurrent = current;
                }

                tocLinks.forEach(link => {
                    link.classList.remove('active');
                    if (link.getAttribute('href') === '#' + current) {
                        link.classList.add('active');
                    }
                });
            }

            content.addEventListener('scroll', updateActive);
            updateActive(); // Initial check
        }
    </script>
</body>
</html>`, lightNovel.Name, chapter.Name, lightNovel.Name, lightNovel.Slug, lightNovel.Slug, chapter.Slug))
}

// HandleLightNovelChapter handles displaying a light novel chapter reader page (like manga chapters)
func HandleLightNovelChapter(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	chapterSlug := c.Params("chapter")

	lightNovel, err := models.GetLightNovel(lightNovelSlug)
	if err != nil {
		return handleError(c, err)
	}
	if lightNovel == nil {
		return handleErrorWithStatus(c, fmt.Errorf("light novel not found or access restricted based on content rating settings"), fiber.StatusNotFound)
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, lightNovel.LibrarySlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	chapters, err := models.GetChapters(lightNovelSlug)
	if err != nil {
		return handleError(c, err)
	}

	chapter, err := models.GetChapter(lightNovelSlug, chapterSlug)
	if err != nil {
		return handleError(c, err)
	}
	if chapter == nil {
		return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
	}

	// Note: chapter is normally marked read by an HTMX trigger in the view.
	// As a safe fallback, if this request is a full page load (not an HTMX request)
	// and the user is logged in, mark the chapter read server-side so the
	// light novel list can reflect the read state for non-HTMX navigation.
	userName := GetUserContext(c)
	if userName != "" && !IsHTMXRequest(c) {
		_ = models.MarkChapterRead(userName, lightNovelSlug, chapterSlug)
	}

	prevSlug, nextSlug, err := models.GetAdjacentChapters(chapter.Slug, lightNovelSlug)
	if err != nil {
		return handleError(c, err)
	}

	// Get TOC and content
	toc := utils.GetTOC(chapter.File)
	content := utils.GetBookContent(chapter.File, lightNovelSlug, chapterSlug)

	// Provide chapters in reverse order for dropdown (newest first) to avoid view-side reversing
	rev := make([]models.Chapter, len(chapters))
	for i := range chapters {
		rev[i] = chapters[len(chapters)-1-i]
	}

	return HandleView(c, views.LightNovelChapter(prevSlug, chapter.Slug, nextSlug, *lightNovel, *chapter, rev, toc, content))
}

// cleanHTMLContent performs basic cleaning of HTML content from EPUB
func cleanHTMLContent(html string) string {
	// Remove DOCTYPE, html, head, body tags
	html = strings.ReplaceAll(html, "<!DOCTYPE html>", "")
	html = strings.ReplaceAll(html, "<html>", "")
	html = strings.ReplaceAll(html, "</html>", "")
	html = strings.ReplaceAll(html, "<head>", "")
	html = strings.ReplaceAll(html, "</head>", "")
	html = strings.ReplaceAll(html, "<body>", "")
	html = strings.ReplaceAll(html, "</body>", "")

	// Remove script and style tags
	for strings.Contains(html, "<script") {
		start := strings.Index(html, "<script")
		end := strings.Index(html[start:], "</script>")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+9:]
	}

	for strings.Contains(html, "<style") {
		start := strings.Index(html, "<style")
		end := strings.Index(html[start:], "</style>")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+8:]
	}

	return html
}

// extractEPUBContent extracts all readable content from an EPUB file
func extractEPUBContent(epubPath string) (EPUBContent, error) {
	content := utils.GetBookContent(epubPath, "", "") // slugs not needed for old extraction
	
	// For now, return a simple structure. We can enhance this later.
	return EPUBContent{
		Title:       "EPUB Content",
		Chapters:    []EPUBChapter{{Title: "Content", Content: content}},
		CSS:         []string{},
		Assets:      make(map[string][]byte),
		Metadata:    make(map[string]string),
	}, nil
}

// HandleLightNovelTOC handles TOC requests for light novel chapters
func HandleLightNovelTOC(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(lightNovelSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", lightNovelSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	toc := utils.GetTOC(chapter.File)
	c.Set("Content-Type", "text/html")
	return c.SendString(toc)
}

// HandleLightNovelBookContent handles book content requests for light novel chapters
func HandleLightNovelBookContent(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(lightNovelSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", lightNovelSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	content := utils.GetBookContent(chapter.File, lightNovelSlug, chapterSlug)
	c.Set("Content-Type", "text/html")
	return c.SendString(content)
}

// HandleLightNovelAsset handles asset requests from EPUB files
func HandleLightNovelAsset(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	chapterSlug := c.Params("chapter")
	assetPath := c.Params("*")

	log.Debugf("Asset request: lightNovel=%s, chapter=%s, assetPath=%s", lightNovelSlug, chapterSlug, assetPath)

	chapter, err := models.GetChapter(lightNovelSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", lightNovelSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		log.Errorf("Chapter not found: %s/%s", lightNovelSlug, chapterSlug)
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		log.Errorf("EPUB file not found: %s", chapter.File)
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	// Open the EPUB file
	r, err := zip.OpenReader(chapter.File)
	if err != nil {
		log.Errorf("Error opening EPUB %s: %v", chapter.File, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error opening EPUB")
	}
	defer r.Close()

	// Find the asset
	var file *zip.File
	for _, f := range r.File {
		if f.Name == assetPath {
			file = f
			break
		}
	}
	if file == nil {
		log.Errorf("Asset not found in EPUB: %s", assetPath)
		return c.Status(fiber.StatusNotFound).SendString("Asset not found")
	}

	rc, err := file.Open()
	if err != nil {
		log.Errorf("Error opening asset %s: %v", assetPath, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error opening asset")
	}
	defer rc.Close()

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(assetPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Set("Content-Type", "image/jpeg")
	case ".png":
		c.Set("Content-Type", "image/png")
	case ".gif":
		c.Set("Content-Type", "image/gif")
	case ".svg":
		c.Set("Content-Type", "image/svg+xml")
	case ".css":
		c.Set("Content-Type", "text/css")
	case ".xhtml", ".html":
		c.Set("Content-Type", "text/html")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}

	log.Debugf("Serving asset %s", assetPath)
	if _, err := io.Copy(c.Response().BodyWriter(), rc); err != nil {
		log.Errorf("Error writing asset %s to response: %v", assetPath, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error writing asset")
	}

	return nil
}

// HandleLightNovelChapterTOCFragment handles TOC fragment requests for light novel chapters
func HandleLightNovelChapterTOCFragment(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(lightNovelSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", lightNovelSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	toc := utils.GetTOC(chapter.File)
	return HandleView(c, views.LightNovelTOCFragment(toc))
}

// HandleLightNovelChapterReaderFragment handles reader fragment requests for light novel chapters
func HandleLightNovelChapterReaderFragment(c *fiber.Ctx) error {
	lightNovelSlug := c.Params("light_novel")
	chapterSlug := c.Params("chapter")

	chapter, err := models.GetChapter(lightNovelSlug, chapterSlug)
	if err != nil {
		log.Errorf("Failed to get chapter %s/%s: %v", lightNovelSlug, chapterSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}
	if chapter == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	// Check library access permission
	hasAccess, err := UserHasLibraryAccess(c, chapter.MangaSlug)
	if err != nil {
		return handleError(c, err)
	}
	if !hasAccess {
		return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
	}

	// Check if the file exists
	if _, err := os.Stat(chapter.File); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("EPUB file not found")
	}

	// Get light novel info
	lightNovel, err := models.GetLightNovel(lightNovelSlug)
	if err != nil {
		log.Errorf("Failed to get light novel %s: %v", lightNovelSlug, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	content := utils.GetBookContent(chapter.File, lightNovelSlug, chapterSlug)

	// Mark chapter as read if user is logged in
	userName := GetUserContext(c)
	if userName != "" {
		err := models.MarkChapterRead(userName, lightNovelSlug, chapterSlug)
		if err != nil {
			log.Warnf("Failed to mark chapter as read: %v", err)
		}
	}

	return HandleView(c, views.LightNovelReaderFragment(*lightNovel, *chapter, content))
}