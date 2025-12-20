package handlers

import (
	"strings"
	"testing"
)

func TestRemoveComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    ".foo { color: red; }",
			expected: ".foo { color: red; }",
		},
		{
			name:     "single comment",
			input:    "/* comment */ .foo { color: red; }",
			expected: " .foo { color: red; }",
		},
		{
			name:     "multiple comments",
			input:    "/* comment1 */ .foo { color: red; } /* comment2 */",
			expected: " .foo { color: red; } ",
		},
		{
			name:     "multiline comment",
			input:    "/* multi\nline\ncomment */ .foo { color: red; }",
			expected: " .foo { color: red; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeComments(tt.input)
			if result != tt.expected {
				t.Errorf("removeComments(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCSSContent(t *testing.T) {
	css := `
:root {
  --color: red;
}

.foo {
  color: var(--color);
}

#bar {
  background: blue;
}

div {
  margin: 0;
}

.baz, .qux {
  padding: 10px;
}

@media (max-width: 768px) {
  .foo {
    color: green;
  }
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
`

	parser := parseCSSContent(css)

	// Check that :root is in universal rules
	if len(parser.UniversalRules) == 0 {
		t.Error("Expected universal rules to contain :root")
	}

	// Check class rules
	if _, ok := parser.RulesByClass["foo"]; !ok {
		t.Error("Expected RulesByClass to contain 'foo'")
	}
	if _, ok := parser.RulesByClass["baz"]; !ok {
		t.Error("Expected RulesByClass to contain 'baz'")
	}
	if _, ok := parser.RulesByClass["qux"]; !ok {
		t.Error("Expected RulesByClass to contain 'qux'")
	}

	// Check ID rules
	if _, ok := parser.RulesByID["bar"]; !ok {
		t.Error("Expected RulesByID to contain 'bar'")
	}

	// Check element rules
	if _, ok := parser.RulesByElement["div"]; !ok {
		t.Error("Expected RulesByElement to contain 'div'")
	}

	// Check at-rules
	if len(parser.AtRules) != 2 {
		t.Errorf("Expected 2 at-rules (media and keyframes), got %d", len(parser.AtRules))
	}
}

func TestExtractRequiredCSS(t *testing.T) {
	// Initialize parser with test CSS
	css := `
:root {
  --color: red;
}

.active {
  color: green;
}

.inactive {
  color: gray;
}

#header {
  height: 60px;
}

#footer {
  height: 40px;
}

button {
  cursor: pointer;
}

span {
  display: inline;
}

@media (max-width: 768px) {
  .active {
    color: blue;
  }
}
`

	cssParserMu.Lock()
	cssParser = parseCSSContent(css)
	cssParserMu.Unlock()

	html := `
<!DOCTYPE html>
<html>
<head></head>
<body>
  <div id="header">
    <button class="active">Click me</button>
  </div>
</body>
</html>
`

	result := ExtractRequiredCSS(html)

	// Should include :root (universal)
	if !strings.Contains(result, ":root") {
		t.Error("Expected result to contain :root")
	}

	// Should include .active
	if !strings.Contains(result, ".active") {
		t.Error("Expected result to contain .active")
	}

	// Should NOT include .inactive (not used in HTML)
	if strings.Contains(result, ".inactive") {
		t.Error("Expected result to NOT contain .inactive")
	}

	// Should include #header
	if !strings.Contains(result, "#header") {
		t.Error("Expected result to contain #header")
	}

	// Should NOT include #footer (not used in HTML)
	if strings.Contains(result, "#footer") {
		t.Error("Expected result to NOT contain #footer")
	}

	// Should include button element
	if !strings.Contains(result, "button") {
		t.Error("Expected result to contain button")
	}

	// Should NOT include span element (not used in HTML)
	// Note: This depends on implementation - some systems include all element rules
	// For strict tree-shaking, we'd want to exclude unused elements

	// Should include @media with .active
	if !strings.Contains(result, "@media") {
		t.Error("Expected result to contain @media query for .active")
	}
}

func TestIsUniversalSelector(t *testing.T) {
	tests := []struct {
		selector string
		expected bool
	}{
		{"*", true},
		{":root", true},
		{"html", true},
		{"body", true},
		{":root { --color: red; }", true},
		{".foo", false},
		{"#bar", false},
		{"div", false},
		{"html body", true},
		{"body .container", true},
	}

	for _, tt := range tests {
		t.Run(tt.selector, func(t *testing.T) {
			result := isUniversalSelector(tt.selector)
			if result != tt.expected {
				t.Errorf("isUniversalSelector(%q) = %v, want %v", tt.selector, result, tt.expected)
			}
		})
	}
}
