package text

import (
	"fmt"
	"html"
	"io"
	"os"
	"strings"
	"sync"

	websocket "github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3/log"
)

// ConsoleLogManager manages WebSocket connections for console log streaming
type ConsoleLogManager struct {
	clients       sync.Map
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
	clients: sync.Map{},
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
		lw.manager.broadcastLog(message)
	}

	return len(p), nil
}

// InitializeConsoleLogger sets up console log capture and streaming
func InitializeConsoleLogger() {
	if consoleLogManager.captureActive {
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

	log.Debug("Console log streaming initialized")
}

// HandleConsoleLogsWebSocket establishes a WebSocket connection for streaming console logs
// Note: Authentication must be validated by the caller before calling this function
func HandleConsoleLogsWebSocket(c *websocket.Conn) {
	// Register the connection
	registerConsoleClient(c)
	defer unregisterConsoleClient(c)

	log.Debug("WebSocket client connected for console logs")

	// Send buffered logs to the new client
	bufferedLogs := consoleLogManager.buffer.GetAll()
	for _, logEntry := range bufferedLogs {
		payload := createLogPayload(logEntry)
		if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Debugf("Failed to send buffered log to client: %v", err)
			return
		}
	}

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
func (clm *ConsoleLogManager) broadcastLog(message string) {
	var conns []*websocket.Conn
	clm.clients.Range(func(key, value any) bool {
		conn := key.(*websocket.Conn)
		conns = append(conns, conn)
		return true
	})
	if len(conns) == 0 {
		return
	}

	payload := createLogPayload(message)

	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			// Client disconnected, clean up
			unregisterConsoleClient(conn)
		}
	}
}

func createLogPayload(message string) []byte {
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

func registerConsoleClient(conn *websocket.Conn) {
	consoleLogManager.clients.Store(conn, struct{}{})
}

func unregisterConsoleClient(conn *websocket.Conn) {
	consoleLogManager.clients.Delete(conn)
	conn.Close() // Close the connection
}
