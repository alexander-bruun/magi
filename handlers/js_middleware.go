package handlers

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// JSMiddleware inlines JavaScript files referenced in HTML responses.
// It trusts the templates to have the correct script tags and simply
// inlines them, preserving load order.
//
// Head-critical scripts (web components like Franken UI) stay in <head>.
// Other scripts are moved to end of <body> for better performance.

var (
	jsCache   = make(map[string]string) // path -> content
	jsCacheMu sync.RWMutex

	// Matches <script src="/assets/js/..."></script>
	scriptTagRegex = regexp.MustCompile(`<script\s+[^>]*src="(/assets/js/[^"]+)"[^>]*>\s*</script>`)

	// Head-critical scripts that must load in <head> before DOM parsing
	// These define custom elements that need to be registered before the parser sees them
	headCriticalScripts = map[string]bool{
		"/assets/js/vendor/core.iife.js": true, // Franken UI core
		"/assets/js/vendor/icon.iife.js": true, // Franken UI icons (uk-icon element)
		"/assets/js/vendor/chart.min.js": true, // Chart.js library for monitoring charts
	}
)

// InitJSCache pre-loads all JS files into memory for fast inlining
func InitJSCache(assetsPath string) error {
	jsDir := filepath.Join(assetsPath, "js")

	err := filepath.Walk(jsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Store with URL path as key
		relPath, _ := filepath.Rel(assetsPath, path)
		urlPath := "/assets/" + filepath.ToSlash(relPath)

		jsCacheMu.Lock()
		jsCache[urlPath] = string(content)
		jsCacheMu.Unlock()

		return nil
	})

	if err != nil {
		return err
	}

	jsCacheMu.RLock()
	count := len(jsCache)
	jsCacheMu.RUnlock()

	log.Infof("JS cache initialized with %d files", count)
	return nil
}

// getJSContent returns cached JS content for a URL path
func getJSContent(urlPath string) (string, bool) {
	jsCacheMu.RLock()
	defer jsCacheMu.RUnlock()
	content, ok := jsCache[urlPath]
	return content, ok
}

// isHeadCritical returns true if the script must be in <head>
func isHeadCritical(urlPath string) bool {
	return headCriticalScripts[urlPath]
}

// JSMiddleware intercepts HTML responses and inlines JavaScript
func JSMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Skip static assets and API calls
		if strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/api/") ||
			strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".map") {
			return c.Next()
		}

		err := c.Next()
		if err != nil {
			return err
		}

		contentType := string(c.Response().Header.ContentType())
		if !strings.Contains(contentType, "text/html") {
			return nil
		}

		body := c.Response().Body()
		if len(body) == 0 {
			return nil
		}

		html := string(body)
		newHTML := processHTML(html)
		c.Response().SetBody([]byte(newHTML))

		return nil
	}
}

// processHTML finds all script tags and inlines them appropriately
func processHTML(html string) string {
	// Find all script tags
	matches := scriptTagRegex.FindAllStringSubmatchIndex(html, -1)
	if len(matches) == 0 {
		return html
	}

	// Separate head-critical from body scripts, preserving order
	var headScripts []scriptInfo
	var bodyScripts []scriptInfo

	for _, match := range matches {
		fullStart, fullEnd := match[0], match[1]
		srcStart, srcEnd := match[2], match[3]
		urlPath := html[srcStart:srcEnd]

		content, ok := getJSContent(urlPath)
		if !ok {
			continue // Keep original tag if we don't have the content
		}

		info := scriptInfo{
			start:   fullStart,
			end:     fullEnd,
			urlPath: urlPath,
			content: content,
		}

		if isHeadCritical(urlPath) {
			headScripts = append(headScripts, info)
		} else {
			bodyScripts = append(bodyScripts, info)
		}
	}

	// Build result by removing script tags and inserting inlined versions
	var result strings.Builder
	lastEnd := 0

	// Process all matches, removing tags we're inlining
	for _, match := range matches {
		fullStart, fullEnd := match[0], match[1]
		srcStart, srcEnd := match[2], match[3]
		urlPath := html[srcStart:srcEnd]

		// Check if we have this script cached
		if _, ok := getJSContent(urlPath); !ok {
			continue // Keep this tag as-is
		}

		result.WriteString(html[lastEnd:fullStart])

		// If this is a head-critical script, inline it here
		if isHeadCritical(urlPath) {
			for _, s := range headScripts {
				if s.urlPath == urlPath && s.start == fullStart {
					result.WriteString("<script>/* ")
					result.WriteString(filepath.Base(urlPath))
					result.WriteString(" */\n;")
					result.WriteString(s.content)
					result.WriteString(";\n</script>")
					break
				}
			}
		}
		// Non-critical scripts are removed here, added at end of body

		lastEnd = fullEnd
	}
	result.WriteString(html[lastEnd:])

	html = result.String()

	// Add body scripts before </body>
	if len(bodyScripts) > 0 {
		var bodyJS strings.Builder
		bodyJS.WriteString("<script id=\"inline-js\">\n")
		for _, s := range bodyScripts {
			bodyJS.WriteString("/* ")
			bodyJS.WriteString(filepath.Base(s.urlPath))
			bodyJS.WriteString(" */\n;")
			bodyJS.WriteString(s.content)
			bodyJS.WriteString(";\n")
		}
		bodyJS.WriteString("</script>\n")

		bodyCloseIdx := strings.LastIndex(html, "</body>")
		if bodyCloseIdx != -1 {
			var finalResult bytes.Buffer
			finalResult.WriteString(html[:bodyCloseIdx])
			finalResult.WriteString(bodyJS.String())
			finalResult.WriteString(html[bodyCloseIdx:])
			html = finalResult.String()
		}
	}

	return html
}

type scriptInfo struct {
	start   int
	end     int
	urlPath string
	content string
}

// GetJSStats returns statistics about cached JavaScript
func GetJSStats() map[string]interface{} {
	jsCacheMu.RLock()
	defer jsCacheMu.RUnlock()

	totalSize := 0
	vendorCount := 0
	appCount := 0

	for path, content := range jsCache {
		totalSize += len(content)
		if strings.Contains(path, "/vendor/") {
			vendorCount++
		} else {
			appCount++
		}
	}

	return map[string]interface{}{
		"totalFiles":     len(jsCache),
		"vendorFiles":    vendorCount,
		"appFiles":       appCount,
		"totalSizeBytes": totalSize,
		"headCritical":   len(headCriticalScripts),
	}
}
