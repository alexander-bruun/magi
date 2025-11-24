package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/alexander-bruun/magi/executor"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	websocket "github.com/gofiber/websocket/v2"
)

// JobStatus represents the current status of a running job
type JobStatus struct {
	Type         string `json:"type"`          // "scraper" or "indexer"
	Name         string `json:"name"`          // Job/script name
	ID           int64  `json:"id"`            // Script ID (for scrapers) or 0 (for indexers)
	LibrarySlug  string `json:"library_slug"`  // Library slug (for indexers only)
	StartTime    int64  `json:"start_time"`    // Unix timestamp when job started
	CurrentMedia string `json:"current_manga"` // Current media being indexed (for indexers)
	Progress     string `json:"progress"`      // Progress information (e.g., "15/100")
}

// JobStatusManager manages WebSocket connections and active job statuses
type JobStatusManager struct {
	clients     map[*websocket.Conn]bool
	activeJobs  map[string]JobStatus // key: unique identifier (e.g., "scraper_1" or "indexer_mylib")
	mu          sync.RWMutex
	writeMu     sync.Mutex // Protects WebSocket writes
	pingTicker  *time.Ticker
	stopPing    chan struct{}
}

var jobStatusManager = &JobStatusManager{
	clients:    make(map[*websocket.Conn]bool),
	activeJobs: make(map[string]JobStatus),
	stopPing:   make(chan struct{}),
}

// Initialize starts the ping routine for keeping WebSocket connections alive
func init() {
	jobStatusManager.pingTicker = time.NewTicker(30 * time.Second)
	go jobStatusManager.pingClients()
}

// pingClients sends periodic pings to all connected clients
func (m *JobStatusManager) pingClients() {
	for {
		select {
		case <-m.pingTicker.C:
			m.mu.RLock()
			clients := make([]*websocket.Conn, 0, len(m.clients))
			for conn := range m.clients {
				clients = append(clients, conn)
			}
			m.mu.RUnlock()

			for _, conn := range clients {
				m.writeMu.Lock()
				err := conn.WriteMessage(websocket.PingMessage, []byte{})
				m.writeMu.Unlock()

				if err != nil {
					log.Debugf("Failed to ping job status client: %v", err)
					m.unregisterClient(conn)
				}
			}
		case <-m.stopPing:
			return
		}
	}
}

// HandleJobStatusWebSocketUpgrade upgrades the connection to WebSocket
func HandleJobStatusWebSocketUpgrade(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return websocket.New(func(conn *websocket.Conn) {
			handleJobStatusWebSocket(conn)
		})(c)
	}
	return c.Status(fiber.StatusUpgradeRequired).SendString("WebSocket upgrade required")
}

// handleJobStatusWebSocket manages the WebSocket connection lifecycle
func handleJobStatusWebSocket(conn *websocket.Conn) {
	jobStatusManager.registerClient(conn)
	defer func() {
		jobStatusManager.unregisterClient(conn)
		log.Debug("Job status WebSocket client disconnected")
	}()

	log.Debug("Job status WebSocket client connected")

	// Set up pong handler
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	// Send current active jobs to the newly connected client
	jobStatusManager.sendActiveJobsToClient(conn)

	// Keep connection open and listen for close
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debugf("Job status WebSocket error: %v", err)
			}
			break
		}
	}
}

// registerClient adds a new WebSocket client
func (m *JobStatusManager) registerClient(conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[conn] = true
}

// unregisterClient removes a WebSocket client
func (m *JobStatusManager) unregisterClient(conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, conn)
	conn.Close()
}

// sendActiveJobsToClient sends the current list of active jobs to a specific client
func (m *JobStatusManager) sendActiveJobsToClient(conn *websocket.Conn) {
	m.mu.RLock()
	jobs := make([]JobStatus, 0, len(m.activeJobs))
	for _, job := range m.activeJobs {
		jobs = append(jobs, job)
	}
	m.mu.RUnlock()

	if len(jobs) > 0 {
		message := map[string]interface{}{
			"action": "jobs_update",
			"jobs":   jobs,
		}
		if data, err := json.Marshal(message); err == nil {
			m.writeMu.Lock()
			conn.WriteMessage(websocket.TextMessage, data)
			m.writeMu.Unlock()
		}
	}
}

// broadcastJobUpdate sends an update to all connected clients
func (m *JobStatusManager) broadcastJobUpdate() {
	m.mu.RLock()
	jobs := make([]JobStatus, 0, len(m.activeJobs))
	for _, job := range m.activeJobs {
		jobs = append(jobs, job)
	}
	clients := make([]*websocket.Conn, 0, len(m.clients))
	for conn := range m.clients {
		clients = append(clients, conn)
	}
	m.mu.RUnlock()

	message := map[string]interface{}{
		"action": "jobs_update",
		"jobs":   jobs,
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Errorf("Failed to marshal job status update: %v", err)
		return
	}

	for _, conn := range clients {
		m.writeMu.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		m.writeMu.Unlock()

		if err != nil {
			log.Debugf("Failed to send job status update: %v", err)
			m.unregisterClient(conn)
		}
	}
}

// NotifyScraperStarted notifies that a scraper script has started
func NotifyScraperStarted(scriptID int64, scriptName string) {
	jobStatusManager.mu.Lock()
	key := getScraperKey(scriptID)
	jobStatusManager.activeJobs[key] = JobStatus{
		Type:      "scraper",
		Name:      scriptName,
		ID:        scriptID,
		StartTime: time.Now().Unix(),
	}
	jobStatusManager.mu.Unlock()

	jobStatusManager.broadcastJobUpdate()
	log.Infof("Notified scraper started: %s (ID=%d)", scriptName, scriptID)
}

// NotifyScraperFinished notifies that a scraper script has finished
func NotifyScraperFinished(scriptID int64) {
	jobStatusManager.mu.Lock()
	key := getScraperKey(scriptID)
	delete(jobStatusManager.activeJobs, key)
	jobStatusManager.mu.Unlock()

	jobStatusManager.broadcastJobUpdate()
	log.Infof("Notified scraper finished: ID=%d", scriptID)
}

// NotifyIndexerStarted notifies that a library indexer has started
func NotifyIndexerStarted(librarySlug string, libraryName string) {
	jobStatusManager.mu.Lock()
	key := getIndexerKey(librarySlug)
	jobStatusManager.activeJobs[key] = JobStatus{
		Type:        "indexer",
		Name:        libraryName,
		LibrarySlug: librarySlug,
		StartTime:   time.Now().Unix(),
	}
	jobStatusManager.mu.Unlock()

	jobStatusManager.broadcastJobUpdate()
	log.Debugf("Notified indexer started: %s (slug=%s)", libraryName, librarySlug)
}

// NotifyIndexerProgress updates the current media being indexed
func NotifyIndexerProgress(librarySlug string, currentMedia string, progress string) {
	jobStatusManager.mu.Lock()
	key := getIndexerKey(librarySlug)
	if job, exists := jobStatusManager.activeJobs[key]; exists {
		job.CurrentMedia = currentMedia
		job.Progress = progress
		jobStatusManager.activeJobs[key] = job
	}
	jobStatusManager.mu.Unlock()

	jobStatusManager.broadcastJobUpdate()
}

// NotifyIndexerFinished notifies that a library indexer has finished
func NotifyIndexerFinished(librarySlug string) {
	jobStatusManager.mu.Lock()
	key := getIndexerKey(librarySlug)
	delete(jobStatusManager.activeJobs, key)
	jobStatusManager.mu.Unlock()

	jobStatusManager.broadcastJobUpdate()
	log.Debugf("Notified indexer finished: slug=%s", librarySlug)
}

// Helper functions to generate unique keys
func getScraperKey(scriptID int64) string {
	return fmt.Sprintf("scraper_%d", scriptID)
}

func getIndexerKey(librarySlug string) string {
	return "indexer_" + librarySlug
}

// HandleScraperLogsWebSocketUpgrade upgrades the connection to WebSocket and extracts the script ID
func HandleScraperLogsWebSocketUpgrade(c *fiber.Ctx) error {
	// Check if this is a WebSocket upgrade request
	if websocket.IsWebSocketUpgrade(c) {
		// Extract script ID from route parameter
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("invalid script id: %v", err))
		}

		// Upgrade to WebSocket with the extracted script ID
		return websocket.New(func(conn *websocket.Conn) {
			executor.HandleLogsWebSocket(conn, id)
		})(c)
	}
	return c.Status(fiber.StatusUpgradeRequired).SendString("WebSocket upgrade required")
}