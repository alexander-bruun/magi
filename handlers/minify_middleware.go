package handlers

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/alexander-bruun/magi/utils/embedded"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

// CSSParser holds parsed CSS data for efficient lookup
type CSSParser struct {
	// All CSS rules as raw strings
	AllRules []string
	// At-rules (media queries, keyframes) - these are included separately
	AtRules []string
	// Universal rules (*, html, body, :root, etc.) - always included
	UniversalRules []string
	// Full CSS content for reference
	FullCSS string
	// Source files that were parsed
	SourceFiles []string
	// Full content of each source file (for explicit includes)
	FileContents map[string]string
}

var (
	cssParser   *CSSParser
	cssParserMu sync.RWMutex

	// JS cache
	jsCache = make(map[string]string) // string -> string, no mutex

	// Minifier instance
	minifier *minify.M

	// Regex patterns
	htmlClassRegex   = regexp.MustCompile(`class="([^"]*)"`)
	htmlIDRegex      = regexp.MustCompile(`id="([^"]*)"`)
	htmlElementRegex = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)[\s>]`)
	cssLinkRegex     = regexp.MustCompile(`<link[^>]+href="(/assets/css/[^"]+\.css)"[^>]*/?>`)
	scriptTagRegex   = regexp.MustCompile(`<script\s+[^>]*src="(/assets/js/[^"]+)"[^>]*>\s*</script>`)

	// Head-critical scripts that must load in <head> before DOM parsing
	headCriticalScripts = map[string]bool{
		"/assets/js/vendor/core.iife.js": true, // Franken UI core
		"/assets/js/vendor/icon.iife.js": true, // Franken UI icons (uk-icon element)
		"/assets/js/vendor/chart.min.js": true, // Chart.js library for monitoring charts
	}
)

// InitMinifier initializes the minifier with CSS, HTML, and JS minifiers
func InitMinifier() {
	minifier = minify.New()
	minifier.AddFunc("text/css", css.Minify)
	minifier.AddFunc("text/html", html.Minify)
	minifier.AddFunc("application/javascript", js.Minify)
	minifier.AddFunc("text/javascript", js.Minify)
}

// InitCSSParser loads and parses all CSS files from the embedded assets filesystem
func InitCSSParser(cssDir string) error {
	cssParserMu.Lock()
	defer cssParserMu.Unlock()

	// Create a new parser that will hold all CSS
	parser := &CSSParser{
		AllRules:       make([]string, 0),
		AtRules:        make([]string, 0),
		UniversalRules: make([]string, 0),
		SourceFiles:    make([]string, 0),
		FileContents:   make(map[string]string),
	}

	var allCSS strings.Builder

	// Walk through the CSS directory in the embedded FS
	err := fs.WalkDir(embedded.Assets, cssDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".css") {
			return nil
		}

		content, err := fs.ReadFile(embedded.Assets, path)
		if err != nil {
			log.Warnf("Failed to read CSS file %s: %v", path, err)
			return nil // Continue with other files
		}

		// Get relative path for tracking
		relPath, _ := filepath.Rel(cssDir, path)
		parser.SourceFiles = append(parser.SourceFiles, relPath)

		// Store file content for explicit includes (strip comments for smaller output)
		parser.FileContents[relPath] = removeComments(string(content))

		// Add file marker comment and content
		allCSS.WriteString("/* Source: " + relPath + " */\n")
		allCSS.WriteString(string(content))
		allCSS.WriteString("\n")

		return nil
	})

	if err != nil {
		return err
	}

	// Parse the combined CSS content
	fullCSS := allCSS.String()
	parser.FullCSS = fullCSS

	// Parse and categorize rules
	parseCSSIntoParser(parser, fullCSS)

	cssParser = parser

	log.Debugf("CSS Parser initialized from %d files: %d rules, %d at-rules, %d universal rules",
		len(parser.SourceFiles),
		len(parser.AllRules),
		len(parser.AtRules),
		len(parser.UniversalRules))

	return nil
}

// InitJSCache pre-loads all JS files into memory for fast inlining
func InitJSCache(assetsPath string) error {
	jsDir := filepath.Join(assetsPath, "js")

	err := fs.WalkDir(embedded.Assets, jsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}

		content, err := fs.ReadFile(embedded.Assets, path)
		if err != nil {
			return err
		}

		// Store content with path starting with /
		relPath, _ := filepath.Rel(filepath.Join(assetsPath, ".."), path)
		jsCache["/"+relPath] = string(content)

		return nil
	})

	if err != nil {
		return err
	}

	log.Debugf("JS Cache initialized with %d files", len(jsCache))
	return nil
}

// MinifyMiddleware intercepts responses and applies minification based on content type
// For HTML: optimizes CSS, inlines JS, then minifies HTML
// For CSS/JS: applies direct minification
func MinifyMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()

		// Skip static assets and API calls (except for direct CSS/JS which we'll minify)
		isStaticAsset := strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/api/") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".ico") ||
			strings.HasSuffix(path, ".woff") ||
			strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".map")

		// Continue with the request
		err := c.Next()
		if err != nil {
			return err
		}

		// Get response body
		body := c.Response().Body()
		if len(body) == 0 {
			return nil
		}

		contentType := string(c.Response().Header.ContentType())

		// Handle different content types
		if strings.Contains(contentType, "text/html") && !isStaticAsset {
			// Process HTML: CSS optimization + JS inlining + HTML minification
			newBody := processHTMLForMinify(string(body), path)

			// Skip HTML minification for home page to improve performance
			if path == "/" {
				c.Response().SetBody([]byte(newBody))
			} else if minifier != nil {
				var buf bytes.Buffer
				if err := minifier.Minify("text/html", &buf, strings.NewReader(newBody)); err == nil {
					c.Response().SetBody(buf.Bytes())
				} else {
					log.Warnf("HTML minification failed: %v", err)
					c.Response().SetBody([]byte(newBody))
				}
			} else {
				c.Response().SetBody([]byte(newBody))
			}
		} else if strings.Contains(contentType, "text/css") {
			// Minify CSS directly
			if minifier != nil {
				var buf bytes.Buffer
				if err := minifier.Minify("text/css", &buf, bytes.NewReader(body)); err == nil {
					c.Response().SetBody(buf.Bytes())
				} else {
					log.Warnf("CSS minification failed: %v", err)
				}
			}
		} else if strings.Contains(contentType, "application/javascript") || strings.Contains(contentType, "text/javascript") {
			// Minify JS directly
			if minifier != nil {
				var buf bytes.Buffer
				if err := minifier.Minify("application/javascript", &buf, bytes.NewReader(body)); err == nil {
					c.Response().SetBody(buf.Bytes())
				} else {
					log.Warnf("JS minification failed: %v", err)
				}
			}
		}

		return nil
	}
}

// processHTMLForMinify combines CSS optimization and JS inlining for HTML responses
func processHTMLForMinify(htmlContent string, _ string) string {
	// First, handle CSS optimization
	htmlContent = processCSSForHTML(htmlContent)

	// Then, handle JS inlining
	htmlContent = processJSForHTML(htmlContent)

	return htmlContent
}

// processCSSForHTML handles CSS optimization for HTML (extracted from original CSSMiddleware)
func processCSSForHTML(bodyStr string) string {
	// Check if this is a full HTML page (has <html> tag)
	isFullPage := strings.Contains(bodyStr, "<html")

	// For partial HTMX responses, we need to handle CSS links differently
	hasCSSLinks := cssLinkRegex.MatchString(bodyStr)

	if !isFullPage {
		// For HTMX partials with CSS links, replace links with inline styles
		if hasCSSLinks {
			// Extract the CSS for the linked files
			var inlineCSS strings.Builder
			cssFileMatches := cssLinkRegex.FindAllStringSubmatch(bodyStr, -1)

			cssParserMu.RLock()
			for _, match := range cssFileMatches {
				if len(match) > 1 && cssParser != nil {
					cssPath := match[1]
					relPath := strings.TrimPrefix(cssPath, "/assets/css/")
					if content, ok := cssParser.FileContents[relPath]; ok {
						inlineCSS.WriteString(content)
						inlineCSS.WriteString("\n")
					}
				}
			}
			cssParserMu.RUnlock()

			// Remove CSS links and add inline style if we have content
			newBody := cssLinkRegex.ReplaceAllString(bodyStr, "")
			if inlineCSS.Len() > 0 {
				styleTag := "<style id=\"htmx-dynamic-css\">\n" + inlineCSS.String() + "</style>\n"
				newBody = styleTag + newBody
			}
			return newBody
		}
		return bodyStr
	}

	// Skip CSS optimization for home page to improve performance
	// if path == "/" {
	// 	return bodyStr
	// }

	// Extract required CSS for full page
	requiredCSS := ExtractRequiredCSS(bodyStr)
	if requiredCSS == "" {
		return bodyStr
	}

	// Create style tag with all required CSS
	styleTag := "<style id=\"dynamic-css\">\n" + requiredCSS + "</style>\n"

	// Remove ALL CSS link tags that point to /assets/css/ and replace with our dynamic style
	firstCSSMatch := cssLinkRegex.FindStringIndex(bodyStr)

	if firstCSSMatch != nil {
		// Remove all CSS link tags
		newBody := cssLinkRegex.ReplaceAllString(bodyStr, "")

		// Find where the first match was and inject our style tag there
		headCloseIdx := strings.Index(newBody, "</head>")
		if headCloseIdx != -1 {
			var result bytes.Buffer
			result.WriteString(newBody[:headCloseIdx])
			result.WriteString(styleTag)
			result.WriteString(newBody[headCloseIdx:])
			return result.String()
		} else {
			return newBody
		}
	} else {
		// No CSS links found, inject before </head>
		headCloseIdx := strings.Index(bodyStr, "</head>")
		if headCloseIdx != -1 {
			var newBody bytes.Buffer
			newBody.WriteString(bodyStr[:headCloseIdx])
			newBody.WriteString(styleTag)
			newBody.WriteString(bodyStr[headCloseIdx:])
			return newBody.String()
		}
	}

	return bodyStr
}

// processJSForHTML handles JS inlining for HTML (extracted from original JSMiddleware)
func processJSForHTML(htmlContent string) string {
	// Skip JS inlining for home page to improve performance
	// if path == "/" {
	// 	return htmlContent
	// }

	// Find all script tags
	matches := scriptTagRegex.FindAllStringSubmatchIndex(htmlContent, -1)
	if len(matches) == 0 {
		return htmlContent
	}

	// Separate head-critical from body scripts, preserving order
	var headScripts []scriptInfo
	var bodyScripts []scriptInfo

	for _, match := range matches {
		fullStart, fullEnd := match[0], match[1]
		srcStart, srcEnd := match[2], match[3]
		urlPath := htmlContent[srcStart:srcEnd]

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
		urlPath := htmlContent[srcStart:srcEnd]

		// Check if we have this script cached
		if _, ok := getJSContent(urlPath); !ok {
			continue // Keep this tag as-is
		}

		result.WriteString(htmlContent[lastEnd:fullStart])

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
	result.WriteString(htmlContent[lastEnd:])

	htmlContent = result.String()

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

		bodyCloseIdx := strings.LastIndex(htmlContent, "</body>")
		if bodyCloseIdx != -1 {
			var finalResult bytes.Buffer
			finalResult.WriteString(htmlContent[:bodyCloseIdx])
			finalResult.WriteString(bodyJS.String())
			finalResult.WriteString(htmlContent[bodyCloseIdx:])
			htmlContent = finalResult.String()
		}
	}

	return htmlContent
}

// Helper functions from original files
func getJSContent(urlPath string) (string, bool) {
	content, ok := jsCache[urlPath]
	return content, ok
}

func isHeadCritical(urlPath string) bool {
	return headCriticalScripts[urlPath]
}

type scriptInfo struct {
	start   int
	end     int
	urlPath string
	content string
}

// ExtractRequiredCSS extracts only the CSS rules needed for the given HTML
func ExtractRequiredCSS(html string) string {
	cssParserMu.RLock()
	defer cssParserMu.RUnlock()

	if cssParser == nil {
		return ""
	}

	// Extract classes, IDs, and elements from HTML
	usedClasses := extractUsedClasses(html)
	usedIDs := extractUsedIDs(html)
	usedElements := extractUsedElements(html)

	// Build CSS with only required rules
	var result strings.Builder

	// Always include universal rules
	for _, rule := range cssParser.UniversalRules {
		result.WriteString(rule)
		result.WriteString("\n")
	}

	// Include rules that match used selectors
	for _, rule := range cssParser.AllRules {
		if ruleMatchesSelectors(rule, usedClasses, usedIDs, usedElements) {
			result.WriteString(rule)
			result.WriteString("\n")
		}
	}

	// Include at-rules that might be needed
	for _, atRule := range cssParser.AtRules {
		// For now, include all at-rules (media queries, keyframes)
		// Could be optimized further by checking if they contain used selectors
		result.WriteString(atRule)
		result.WriteString("\n")
	}

	return result.String()
}

// Helper functions for CSS extraction (copied from original)
func extractUsedClasses(html string) map[string]bool {
	classes := make(map[string]bool)
	matches := htmlClassRegex.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 1 {
			classList := strings.FieldsSeq(match[1])
			for class := range classList {
				classes[class] = true
			}
		}
	}
	return classes
}

func extractUsedIDs(html string) map[string]bool {
	ids := make(map[string]bool)
	matches := htmlIDRegex.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 1 {
			ids[match[1]] = true
		}
	}
	return ids
}

func extractUsedElements(html string) map[string]bool {
	elements := make(map[string]bool)
	matches := htmlElementRegex.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 1 {
			elements[strings.ToLower(match[1])] = true
		}
	}
	return elements
}

func ruleMatchesSelectors(rule string, classes map[string]bool, ids map[string]bool, elements map[string]bool) bool {
	before, _, ok := strings.Cut(rule, "{")
	if !ok {
		return false
	}

	selector := strings.TrimSpace(before)

	// Check for class selectors
	for class := range classes {
		if strings.Contains(selector, "."+class) {
			return true
		}
	}

	// Check for ID selectors
	for id := range ids {
		if strings.Contains(selector, "#"+id) {
			return true
		}
	}

	// Check for element selectors
	for element := range elements {
		if strings.Contains(selector, element) {
			return true
		}
	}

	return false
}

// parseCSSIntoParser and other CSS parsing functions (copied from original)
func parseCSSIntoParser(parser *CSSParser, css string) {
	// Remove comments
	css = removeComments(css)

	// Parse rules
	parseRulesIntoParser(parser, css)
}

func parseRulesIntoParser(parser *CSSParser, css string) {
	i := 0
	for i < len(css) {
		// Skip whitespace
		for i < len(css) && (css[i] == ' ' || css[i] == '\t' || css[i] == '\n' || css[i] == '\r') {
			i++
		}
		if i >= len(css) {
			break
		}

		// Check for at-rules (@media, @keyframes, etc.)
		if css[i] == '@' {
			atRule, endPos := parseAtRule(css[i:])
			if atRule != "" {
				// For @media rules, parse the inner content
				if strings.HasPrefix(atRule, "@media") || strings.HasPrefix(atRule, "@supports") ||
					strings.HasPrefix(atRule, "@keyframes") || strings.HasPrefix(atRule, "@-webkit-keyframes") {
					parser.AtRules = append(parser.AtRules, atRule)
				} else if strings.HasPrefix(atRule, "@font-face") {
					parser.UniversalRules = append(parser.UniversalRules, atRule)
				} else if strings.HasPrefix(atRule, "@import") {
					// Skip @import rules - they should be resolved already
				} else if strings.HasPrefix(atRule, "@charset") {
					// Skip @charset rules
				} else {
					// Other at-rules go to universal
					parser.UniversalRules = append(parser.UniversalRules, atRule)
				}
			}
			i += endPos
			continue
		}

		// Parse regular rule
		rule, endPos := parseRule(css[i:])
		if rule != "" {
			// Check if this is a universal rule
			before, _, ok := strings.Cut(rule, "{")
			if ok {
				selector := strings.TrimSpace(before)
				// Check for :root, *, html, body as primary selectors
				if selector == "*" || selector == ":root" || selector == "html" || selector == "body" ||
					strings.HasPrefix(selector, ":root") || strings.HasPrefix(selector, "html") ||
					strings.HasPrefix(selector, "body") || strings.HasPrefix(selector, "*") {
					parser.UniversalRules = append(parser.UniversalRules, rule)
				} else {
					parser.AllRules = append(parser.AllRules, rule)
				}
			}
		}
		if endPos == 0 {
			i++
		} else {
			i += endPos
		}
	}
}

func parseAtRule(css string) (string, int) {
	i := 0
	braceCount := 0
	inString := false
	stringChar := byte(0)

	for i < len(css) {
		char := css[i]

		if inString {
			if char == stringChar && (i == 0 || css[i-1] != '\\') {
				inString = false
				stringChar = 0
			}
			i++
			continue
		}

		switch char {
		case '"', '\'':
			inString = true
			stringChar = char
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				return css[:i+1], i + 1
			}
		case ';':
			if braceCount == 0 {
				return css[:i+1], i + 1
			}
		}
		i++
	}

	return "", 0
}

func parseRule(css string) (string, int) {
	i := 0
	braceCount := 0
	inString := false
	stringChar := byte(0)

	for i < len(css) {
		char := css[i]

		if inString {
			if char == stringChar && (i == 0 || css[i-1] != '\\') {
				inString = false
				stringChar = 0
			}
			i++
			continue
		}

		switch char {
		case '"', '\'':
			inString = true
			stringChar = char
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				return css[:i+1], i + 1
			}
		}
		i++
	}

	return "", 0
}

func removeComments(css string) string {
	var result strings.Builder
	i := 0
	for i < len(css) {
		if i < len(css)-1 && css[i] == '/' && css[i+1] == '*' {
			// Start of comment
			i += 2
			for i < len(css)-1 && !(css[i] == '*' && css[i+1] == '/') {
				i++
			}
			if i < len(css)-1 {
				i += 2 // Skip */
			}
		} else {
			result.WriteByte(css[i])
			i++
		}
	}
	return result.String()
}

// GetMinifyStats returns statistics about the minifier and caches
func GetMinifyStats() map[string]any {
	cssStats := GetCSSStats()
	jsStats := GetJSStats()

	return map[string]any{
		"css":             cssStats,
		"js":              jsStats,
		"minifierEnabled": minifier != nil,
	}
}

// GetCSSStats returns statistics about parsed CSS
func GetCSSStats() map[string]int {
	cssParserMu.RLock()
	defer cssParserMu.RUnlock()

	if cssParser == nil {
		return nil
	}

	return map[string]int{
		"rules":          len(cssParser.AllRules),
		"atRules":        len(cssParser.AtRules),
		"universalRules": len(cssParser.UniversalRules),
	}
}

// GetJSStats returns statistics about cached JavaScript
func GetJSStats() map[string]any {
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

	return map[string]any{
		"totalFiles":     len(jsCache),
		"vendorFiles":    vendorCount,
		"appFiles":       appCount,
		"totalSizeBytes": totalSize,
		"headCritical":   len(headCriticalScripts),
	}
}
