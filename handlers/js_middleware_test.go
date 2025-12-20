package handlers

import (
	"strings"
	"testing"
)

func TestProcessHTML_NoScripts(t *testing.T) {
	html := "<html><body>Hello</body></html>"
	result := processHTML(html)
	if result != html {
		t.Errorf("Expected unchanged HTML, got %s", result)
	}
}

func TestProcessHTML_InlineBodyScript(t *testing.T) {
	// Pre-populate the cache
	jsCacheMu.Lock()
	jsCache["/assets/js/test.js"] = "console.log('test');"
	jsCacheMu.Unlock()

	html := `<html><head></head><body><script src="/assets/js/test.js"></script></body></html>`
	result := processHTML(html)

	// The script tag should be removed and content inlined before </body>
	if strings.Contains(result, `src="/assets/js/test.js"`) {
		t.Error("Script tag should be removed")
	}
	if !strings.Contains(result, "console.log('test');") {
		t.Error("Script content should be inlined")
	}
	if !strings.Contains(result, "<script id=\"inline-js\">") {
		t.Error("Should have inline-js script tag")
	}

	// Cleanup
	jsCacheMu.Lock()
	delete(jsCache, "/assets/js/test.js")
	jsCacheMu.Unlock()
}

func TestProcessHTML_HeadCriticalScript(t *testing.T) {
	// Pre-populate the cache
	jsCacheMu.Lock()
	jsCache["/assets/js/vendor/core.iife.js"] = "/* franken ui */"
	jsCacheMu.Unlock()

	html := `<html><head><script src="/assets/js/vendor/core.iife.js"></script></head><body></body></html>`
	result := processHTML(html)

	// The script should be inlined in head
	if strings.Contains(result, `src="/assets/js/vendor/core.iife.js"`) {
		t.Error("Script tag should be replaced")
	}
	if !strings.Contains(result, "/* franken ui */") {
		t.Error("Script content should be inlined in head")
	}

	// Cleanup
	jsCacheMu.Lock()
	delete(jsCache, "/assets/js/vendor/core.iife.js")
	jsCacheMu.Unlock()
}
