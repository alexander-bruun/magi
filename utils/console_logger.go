package utils

import (
	"fmt"
	"html"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2/log"
	websocket "github.com/gofiber/websocket/v2"
)

// ConsoleLogManager manages WebSocket connections for console log streaming
type ConsoleLogManager struct {
	clients       []*websocket.Conn
	mu            sync.RWMutex
	buffer        *logBuffer
	captureActive bool
}

type logBuffer struct {
	mu      sync.Mutex
	entries []string
	maxSize int
}

func newLogBuffer(maxSize int) *logBuffer {
	return &logBuffer{
		entries: make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

func (lb *logBuffer) Add(entry string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.entries = append(lb.entries, entry)
	if len(lb.entries) > lb.maxSize {
		lb.entries = lb.entries[1:]
	}
}

func (lb *logBuffer) GetAll() []string {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	result := make([]string, len(lb.entries))
	copy(result, lb.entries)
	return result
}

var consoleLogManager = &ConsoleLogManager{
	clients: make([]*websocket.Conn, 0),
	buffer:  newLogBuffer(1000), // Keep last 1000 log entries
}

// logWriter wraps an io.Writer to broadcast logs
type logWriter struct {
	manager *ConsoleLogManager
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	// Broadcast to websockets
	message := string(p)
	message = strings.TrimRight(message, "\n")
	
	if message != "" {
		// Store in buffer (message already contains timestamp from logger)
		lw.manager.buffer.Add(message)
		
		// Broadcast to connected clients
		lw.manager.broadcastLog("info", message)
	}

	return len(p), nil
}

// InitializeConsoleLogger sets up console log capture and streaming
func InitializeConsoleLogger() {
	consoleLogManager.mu.Lock()
	
	if consoleLogManager.captureActive {
		consoleLogManager.mu.Unlock()
		return
	}

	// Create wrapped writer that broadcasts to websockets
	writer := &logWriter{
		manager: consoleLogManager,
	}

	// Create a multi-writer that writes to both stdout and our broadcaster
	multiWriter := io.MultiWriter(os.Stdout, writer)
	
	// Set the log output to use our multi-writer
	log.SetOutput(multiWriter)

	consoleLogManager.captureActive = true
	consoleLogManager.mu.Unlock()
	
	log.Debug("Console log streaming initialized")
}

// HandleConsoleLogsWebSocket establishes a WebSocket connection for streaming console logs
// Note: Authentication must be validated by the caller before calling this function
func HandleConsoleLogsWebSocket(c *websocket.Conn) {
	// Register the connection
	registerConsoleClient(c)
	defer unregisterConsoleClient(c)

	log.Debug("WebSocket client connected for console logs")

	// Only stream new logs, do not send buffered logs

	// Keep connection alive and wait for close
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debugf("WebSocket closed unexpectedly: %v", err)
			}
			break
		}
	}
}

// broadcastLog sends a log message to all connected clients
func (clm *ConsoleLogManager) broadcastLog(logType string, message string) {
	clm.mu.RLock()
	if len(clm.clients) == 0 {
		clm.mu.RUnlock()
		return
	}
	conns := make([]*websocket.Conn, len(clm.clients))
	copy(conns, clm.clients)
	clm.mu.RUnlock()

	payload := createLogPayload(logType, message)

	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			// Client disconnected, clean up
			unregisterConsoleClient(conn)
		}
	}
}

func createLogPayload(logType string, message string) []byte {
	// Create HTML for HTMX WebSocket extension
	// Send content that will be appended to #console-logs-output
	// Using a wrapper with hx-swap-oob to ensure HTMX processes it correctly
	escapedMessage := html.EscapeString(message)
	// Replace newlines with <br> tags to preserve line breaks in HTML
	escapedMessage = strings.ReplaceAll(escapedMessage, "\n", "<br>")
	// Send a template fragment that HTMX can parse and inject
	html := fmt.Sprintf(`<div id="log-entry" hx-swap-oob="beforeend:#console-logs-output"><div style="white-space:pre-wrap; word-break:break-word; min-height:1.2em;">%s</div></div>`, escapedMessage)
	return []byte(html)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func registerConsoleClient(conn *websocket.Conn) {
	consoleLogManager.mu.Lock()
	defer consoleLogManager.mu.Unlock()
	consoleLogManager.clients = append(consoleLogManager.clients, conn)
}

func unregisterConsoleClient(conn *websocket.Conn) {
	consoleLogManager.mu.Lock()
	defer consoleLogManager.mu.Unlock()

	for i, c := range consoleLogManager.clients {
		if c == conn {
			consoleLogManager.clients = append(consoleLogManager.clients[:i], consoleLogManager.clients[i+1:]...)
			conn.Close() // Close the connection
			break
		}
	}
}
