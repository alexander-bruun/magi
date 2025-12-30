package handlers

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
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
)

// htmlClassRegex matches class attributes in HTML
var htmlClassRegex = regexp.MustCompile(`class="([^"]*)"`)

// htmlIDRegex matches id attributes in HTML
var htmlIDRegex = regexp.MustCompile(`id="([^"]*)"`)

// htmlElementRegex matches HTML elements
var htmlElementRegex = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)[\s>]`)

// cssLinkRegex matches CSS link tags
var cssLinkRegex = regexp.MustCompile(`<link[^>]+href="(/assets/css/[^"]+\.css)"[^>]*/?>`)

// InitCSSParser loads and parses all CSS files from the given directory
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

	// Walk through the CSS directory and parse all CSS files
	err := filepath.Walk(cssDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".css") {
			return nil
		}

		content, err := os.ReadFile(path)
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

	log.Infof("CSS Parser initialized from %d files: %d rules, %d at-rules, %d universal rules",
		len(parser.SourceFiles),
		len(parser.AllRules),
		len(parser.AtRules),
		len(parser.UniversalRules))

	return nil
}

// parseCSSIntoParser parses CSS content and adds rules to the parser
func parseCSSIntoParser(parser *CSSParser, css string) {
	// Remove comments
	css = removeComments(css)

	// Parse rules
	parseRulesIntoParser(parser, css)
}

// parseRulesIntoParser parses CSS rules and adds them to an existing parser
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
			braceIdx := strings.Index(rule, "{")
			if braceIdx != -1 {
				selector := strings.TrimSpace(rule[:braceIdx])
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

// parseCSSContent parses CSS content into structured rules (used by tests)
func parseCSSContent(css string) *CSSParser {
	parser := &CSSParser{
		AllRules:       make([]string, 0),
		AtRules:        make([]string, 0),
		UniversalRules: make([]string, 0),
		FullCSS:        css,
		SourceFiles:    make([]string, 0),
		FileContents:   make(map[string]string),
	}

	// Remove comments
	css = removeComments(css)

	// Parse rules
	parseRulesIntoParser(parser, css)

	return parser
}

// removeComments removes CSS comments
func removeComments(css string) string {
	var result strings.Builder
	i := 0
	for i < len(css) {
		if i+1 < len(css) && css[i] == '/' && css[i+1] == '*' {
			// Find end of comment
			end := strings.Index(css[i+2:], "*/")
			if end == -1 {
				break
			}
			i = i + 2 + end + 2
		} else {
			result.WriteByte(css[i])
			i++
		}
	}
	return result.String()
}

// isUniversalSelector checks if selector is universal (*, :root, html, body)
func isUniversalSelector(selector string) bool {
	selector = strings.TrimSpace(selector)
	// Check for :root, *, html, body as primary selectors
	if selector == "*" || selector == ":root" || selector == "html" || selector == "body" {
		return true
	}
	// Check if starts with these
	if strings.HasPrefix(selector, ":root") ||
		strings.HasPrefix(selector, "html") ||
		strings.HasPrefix(selector, "body") ||
		strings.HasPrefix(selector, "*") {
		return true
	}
	return false
}

// parseAtRule extracts an at-rule including its block
func parseAtRule(css string) (string, int) {
	if len(css) == 0 || css[0] != '@' {
		return "", 0
	}

	braceCount := 0
	inBraces := false
	i := 0

	for i < len(css) {
		if css[i] == '{' {
			braceCount++
			inBraces = true
		} else if css[i] == '}' {
			braceCount--
			if inBraces && braceCount == 0 {
				return strings.TrimSpace(css[:i+1]), i + 1
			}
		}
		i++
	}

	return "", i
}

// parseRule extracts a single CSS rule (selector + block)
func parseRule(css string) (string, int) {
	braceStart := strings.Index(css, "{")
	if braceStart == -1 {
		return "", len(css)
	}

	braceCount := 1
	i := braceStart + 1

	for i < len(css) && braceCount > 0 {
		if css[i] == '{' {
			braceCount++
		} else if css[i] == '}' {
			braceCount--
		}
		i++
	}

	if braceCount == 0 {
		return strings.TrimSpace(css[:i]), i
	}

	return "", len(css)
}

// ExtractRequiredCSS analyzes HTML and returns only the CSS needed
func ExtractRequiredCSS(html string) string {
	cssParserMu.RLock()
	defer cssParserMu.RUnlock()

	if cssParser == nil {
		return ""
	}

	var result strings.Builder
	seenRules := make(map[string]bool)
	seenFiles := make(map[string]bool)

	// First, find any explicitly linked CSS files and include their full content
	// These are files that the page specifically requests (e.g., cropper.min.css)
	cssFileMatches := cssLinkRegex.FindAllStringSubmatch(html, -1)
	for _, match := range cssFileMatches {
		if len(match) > 1 {
			// Extract file path from /assets/css/...
			cssPath := match[1]
			// Convert to relative path that matches our FileContents keys
			relPath := strings.TrimPrefix(cssPath, "/assets/css/")

			if content, ok := cssParser.FileContents[relPath]; ok && !seenFiles[relPath] {
				result.WriteString("/* Explicit include: " + relPath + " */\n")
				result.WriteString(content)
				result.WriteString("\n")
				seenFiles[relPath] = true
			}
		}
	}

	// Always include universal rules
	for _, rule := range cssParser.UniversalRules {
		if !seenRules[rule] {
			result.WriteString(rule)
			result.WriteString("\n")
			seenRules[rule] = true
		}
	}

	// Extract used selectors from HTML
	usedClasses := make(map[string]bool)
	usedIDs := make(map[string]bool)
	usedElements := make(map[string]bool)

	// Extract classes from HTML
	classMatches := htmlClassRegex.FindAllStringSubmatch(html, -1)
	for _, match := range classMatches {
		if len(match) > 1 {
			classes := strings.Fields(match[1])
			for _, class := range classes {
				usedClasses[class] = true
			}
		}
	}

	// Note: The following logging of missing classes is commented out to reduce log noise. It is strictly used for debugging purposes to clean up non-existing CSS classes.
	// Log classes that don't exist in CSS files
	// for class := range usedClasses {
	// 	if !strings.Contains(cssParser.FullCSS, "."+class) {
	// 		if !strings.Contains(class, "${") {
	// 			if !strings.Contains(class, ":") {
	// 				if strings.HasPrefix(class, "lucide-") {
	// 					continue // Skip UIkit classes
	// 				}
	// 				if class == "internal-htmx-wrapper" || class == "uk-slider-nav" || class == "uk-lightbox-slidenav" || class == "rounded-lgw-2/4" || class == "uk-text-large" || class == "uk-lightbox-close" || class == "uk-margin-small" || class == "uk-position-medium" || class == "uk-close-large" || class == "uk-position-small" || class == "uk-custom-select-check" || class == "lucide" || class == "uk-margin-small-top" || class == "uk-modal-close" || class == "theme-option" || class == "uk-lsh" || class == "uk-lightbox-counter" {
	// 					continue // Skip internal HTMX wrapper class
	// 				}
	// 				log.Infof("CSS class '%s' used in HTML but not found in any CSS files", class)
	// 			}
	// 		}
	// 	}
	// }

	// Extract IDs from HTML
	idMatches := htmlIDRegex.FindAllStringSubmatch(html, -1)
	for _, match := range idMatches {
		if len(match) > 1 {
			usedIDs[match[1]] = true
		}
	}

	// Extract elements from HTML
	elementMatches := htmlElementRegex.FindAllStringSubmatch(html, -1)
	for _, match := range elementMatches {
		if len(match) > 1 {
			usedElements[strings.ToLower(match[1])] = true
		}
	}

	// Check each rule to see if it matches any used selectors
	for _, rule := range cssParser.AllRules {
		if seenRules[rule] {
			continue
		}

		// Extract selector from rule
		braceIdx := strings.Index(rule, "{")
		if braceIdx == -1 {
			continue
		}
		selector := strings.TrimSpace(rule[:braceIdx])

		// Check if this rule should be included
		shouldInclude := false

		// Check classes
		for class := range usedClasses {
			if strings.Contains(selector, "."+class) {
				shouldInclude = true
				break
			}
		}
		if shouldInclude {
			result.WriteString(rule)
			result.WriteString("\n")
			seenRules[rule] = true
			continue
		}

		// Check IDs
		for id := range usedIDs {
			if strings.Contains(selector, "#"+id) {
				shouldInclude = true
				break
			}
		}
		if shouldInclude {
			result.WriteString(rule)
			result.WriteString("\n")
			seenRules[rule] = true
			continue
		}

		// Check elements
		for element := range usedElements {
			if strings.Contains(selector, element) {
				shouldInclude = true
				break
			}
		}
		if shouldInclude {
			result.WriteString(rule)
			result.WriteString("\n")
			seenRules[rule] = true
		}
	}

	// Include at-rules (media queries, keyframes) that reference used classes/IDs
	for _, atRule := range cssParser.AtRules {
		// Check if any used class appears in the at-rule
		shouldInclude := false
		for class := range usedClasses {
			if strings.Contains(atRule, "."+class) {
				shouldInclude = true
				break
			}
		}
		if !shouldInclude {
			for id := range usedIDs {
				if strings.Contains(atRule, "#"+id) {
					shouldInclude = true
					break
				}
			}
		}
		if !shouldInclude {
			for element := range usedElements {
				if strings.Contains(atRule, element) {
					shouldInclude = true
					break
				}
			}
		}

		if shouldInclude && !seenRules[atRule] {
			result.WriteString(atRule)
			result.WriteString("\n")
			seenRules[atRule] = true
		}
	}

	return result.String()
}

// CSSMiddleware intercepts responses and injects only required CSS
func CSSMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only process HTML responses
		path := c.Path()

		// Skip static assets
		if strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/api/") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".ico") ||
			strings.HasSuffix(path, ".woff") ||
			strings.HasSuffix(path, ".woff2") {
			return c.Next()
		}

		// Continue with the request
		err := c.Next()
		if err != nil {
			return err
		}

		// Check if response is HTML
		contentType := string(c.Response().Header.ContentType())
		if !strings.Contains(contentType, "text/html") {
			return nil
		}

		// Get the response body
		body := c.Response().Body()
		if len(body) == 0 {
			return nil
		}

		bodyStr := string(body)

		// Check if this is a full HTML page (has <html> tag)
		isFullPage := strings.Contains(bodyStr, "<html")

		// For partial HTMX responses, we need to handle CSS links differently
		// Check if there are any CSS links in the response
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
				c.Response().SetBody([]byte(newBody))
			}
			return nil
		}

		// Extract required CSS for full page
		requiredCSS := ExtractRequiredCSS(bodyStr)
		if requiredCSS == "" {
			return nil
		}

		// Create style tag with all required CSS
		styleTag := "<style id=\"dynamic-css\">\n" + requiredCSS + "</style>\n"

		// Remove ALL CSS link tags that point to /assets/css/ and replace with our dynamic style
		// First, find the first CSS link to know where to inject our style tag
		firstCSSMatch := cssLinkRegex.FindStringIndex(bodyStr)

		if firstCSSMatch != nil {
			// Remove all CSS link tags
			newBody := cssLinkRegex.ReplaceAllString(bodyStr, "")

			// Find where the first match was and inject our style tag there
			// Since we removed content, we need to find </head> or use original position
			headCloseIdx := strings.Index(newBody, "</head>")
			if headCloseIdx != -1 {
				var result bytes.Buffer
				result.WriteString(newBody[:headCloseIdx])
				result.WriteString(styleTag)
				result.WriteString(newBody[headCloseIdx:])
				c.Response().SetBody(result.Bytes())
			} else {
				c.Response().SetBody([]byte(newBody))
			}
		} else {
			// No CSS links found, inject before </head>
			headCloseIdx := strings.Index(bodyStr, "</head>")
			if headCloseIdx != -1 {
				var newBody bytes.Buffer
				newBody.WriteString(bodyStr[:headCloseIdx])
				newBody.WriteString(styleTag)
				newBody.WriteString(bodyStr[headCloseIdx:])
				c.Response().SetBody(newBody.Bytes())
			}
		}

		return nil
	}
}

// GetFullCSS returns the full parsed CSS (for debugging)
func GetFullCSS() string {
	cssParserMu.RLock()
	defer cssParserMu.RUnlock()

	if cssParser == nil {
		return ""
	}
	return cssParser.FullCSS
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
