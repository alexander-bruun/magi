package utils

import (
	"testing"

	"github.com/gofiber/websocket/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewLogBuffer(t *testing.T) {
	lb := newLogBuffer(10)
	assert.NotNil(t, lb)
	assert.Equal(t, 10, lb.maxSize)
	assert.Empty(t, lb.entries)
}

func TestLogBufferAdd(t *testing.T) {
	lb := newLogBuffer(3)

	// Add entries
	lb.Add("log1")
	lb.Add("log2")
	lb.Add("log3")

	assert.Equal(t, []string{"log1", "log2", "log3"}, lb.entries)

	// Add one more, should evict oldest
	lb.Add("log4")
	assert.Equal(t, []string{"log2", "log3", "log4"}, lb.entries)
}

func TestLogBufferGetAll(t *testing.T) {
	lb := newLogBuffer(10)
	lb.Add("log1")
	lb.Add("log2")

	result := lb.GetAll()
	assert.Equal(t, []string{"log1", "log2"}, result)

	// Modify original should not affect copy
	lb.Add("log3")
	assert.Equal(t, []string{"log1", "log2"}, result)
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special chars",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "quotes",
			input:    `he said "hello"`,
			expected: `he said \"hello\"`,
		},
		{
			name:     "backslashes",
			input:    `path\to\file`,
			expected: `path\\to\\file`,
		},
		{
			name:     "newlines",
			input:    "line1\nline2",
			expected: "line1\\nline2",
		},
		{
			name:     "tabs",
			input:    "col1\tcol2",
			expected: "col1\\tcol2",
		},
		{
			name:     "carriage returns",
			input:    "line1\r\nline2",
			expected: "line1\\r\\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateLogPayload(t *testing.T) {
	payload := createLogPayload("info", "test message")
	expected := `<div id="log-entry" hx-swap-oob="beforeend:#console-logs-output"><div style="white-space:pre-wrap; word-break:break-word; min-height:1.2em;">test message</div></div>`
	assert.Equal(t, expected, string(payload))
}

func TestCreateLogPayloadWithEscaping(t *testing.T) {
	payload := createLogPayload("error", `message with "quotes" and \backslashes`)
	expected := `<div id="log-entry" hx-swap-oob="beforeend:#console-logs-output"><div style="white-space:pre-wrap; word-break:break-word; min-height:1.2em;">message with &#34;quotes&#34; and \backslashes</div></div>`
	assert.Equal(t, expected, string(payload))
}

func TestCreateLogPayloadWithNewlines(t *testing.T) {
	payload := createLogPayload("info", "line 1\nline 2\nline 3")
	expected := `<div id="log-entry" hx-swap-oob="beforeend:#console-logs-output"><div style="white-space:pre-wrap; word-break:break-word; min-height:1.2em;">line 1<br>line 2<br>line 3</div></div>`
	assert.Equal(t, expected, string(payload))
}

func TestLogWriterWrite(t *testing.T) {
	// Create a test manager with buffer
	manager := &ConsoleLogManager{
		buffer:  newLogBuffer(10),
		clients: make([]*websocket.Conn, 0),
	}

	writer := &logWriter{manager: manager}

	// Test writing a message
	message := "test log message\n"
	n, err := writer.Write([]byte(message))

	assert.NoError(t, err)
	assert.Equal(t, len(message), n)

	// Check that message was added to buffer (without trailing newline)
	assert.Equal(t, []string{"test log message"}, manager.buffer.GetAll())
}

func TestLogWriterWriteEmpty(t *testing.T) {
	// Create a test manager with buffer
	manager := &ConsoleLogManager{
		buffer:  newLogBuffer(10),
		clients: make([]*websocket.Conn, 0),
	}

	writer := &logWriter{manager: manager}

	// Test writing just a newline (should be ignored)
	n, err := writer.Write([]byte("\n"))
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Empty(t, manager.buffer.GetAll())
}

func TestLogWriterWriteMultipleLines(t *testing.T) {
	// Create a test manager with buffer
	manager := &ConsoleLogManager{
		buffer:  newLogBuffer(10),
		clients: make([]*websocket.Conn, 0),
	}

	writer := &logWriter{manager: manager}

	// Test writing multiple lines - Write method treats this as one message
	message := "line1\nline2\nline3"
	n, err := writer.Write([]byte(message))

	assert.NoError(t, err)
	assert.Equal(t, len(message), n)

	// Should have 1 entry (Write doesn't split on newlines)
	expected := []string{"line1\nline2\nline3"}
	assert.Equal(t, expected, manager.buffer.GetAll())
}