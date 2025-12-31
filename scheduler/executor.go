package scheduler

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/log"
	websocket "github.com/gofiber/websocket/v2"

	"github.com/alexander-bruun/magi/models"
)

// SubscriptionExpiryJob checks for expired subscriptions and downgrades users
type SubscriptionExpiryJob struct{}

func (j *SubscriptionExpiryJob) Execute() error {
	log.Debug("Running subscription expiry check")

	// Get all users with expired active subscriptions
	expiredUsers, err := models.GetExpiredSubscriptions()
	if err != nil {
		log.Errorf("Failed to get expired subscriptions: %v", err)
		return err
	}

	// Downgrade expired users
	for _, username := range expiredUsers {
		log.Infof("Downgrading user %s from premium to reader (subscription expired)", username)
		if err := models.UpdateUserRoleToReader(username); err != nil {
			log.Errorf("Failed to downgrade user %s: %v", username, err)
		} else {
			// Mark subscription as expired
			if err := models.ExpireSubscription(username); err != nil {
				log.Errorf("Failed to mark subscription as expired for user %s: %v", username, err)
			}
		}
	}

	if len(expiredUsers) > 0 {
		log.Infof("Processed %d expired subscriptions", len(expiredUsers))
	} else {
		log.Debug("No expired subscriptions found")
	}

	return nil
}

func (j *SubscriptionExpiryJob) Name() string {
	return "subscription_expiry_check"
}

// LogStreamManager manages WebSocket connections for log streaming
type LogStreamManager struct {
	clients map[string][]*websocket.Conn // key -> list of connections
	mu      sync.RWMutex
}

var logStreamManager = &LogStreamManager{
	clients: make(map[string][]*websocket.Conn),
}

// Execution contexts for running scripts (to allow cancellation)
var execContexts = struct {
	mu    sync.RWMutex
	funcs map[int64]context.CancelFunc
}{
	funcs: make(map[int64]context.CancelFunc),
}

// HandleLogsWebSocket establishes a WebSocket connection for streaming logs
func HandleLogsWebSocket(c *websocket.Conn, key string) {
	// Register the connection
	registerClient(key, c)
	defer func() {
		unregisterClient(key, c)
		log.Debugf("WebSocket client disconnected for key %s", key)
	}()

	log.Debugf("WebSocket client connected for key %s", key)

	// Set up pong handler to detect connection health
	c.SetPongHandler(func(string) error {
		log.Debugf("Received pong from client for key %s", key)
		return nil
	})

	// Keep connection alive and wait for close
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Debugf("WebSocket closed for key %s: %v", key, err)
			}
			break
		}
	}
}

// ansiToHTML converts ANSI escape codes in a string to HTML font tags for colored display
func ansiToHTML(input string) string {
	// Replace ANSI escape codes with HTML font tags
	// Common ANSI codes: \x1b[1;34m (blue), \x1b[1;32m (green), \x1b[1;33m (yellow), \x1b[1;31m (red), \x1b[0m (reset)

	// Blue for INFO
	input = strings.ReplaceAll(input, "\x1b[1;34m", `<font style="color: #3b82f6; display: inline;">`)
	// Green for SUCCESS
	input = strings.ReplaceAll(input, "\x1b[1;32m", `<font style="color: #10b981; display: inline;">`)
	// Yellow for WARNING
	input = strings.ReplaceAll(input, "\x1b[1;33m", `<font style="color: #f59e0b; display: inline;">`)
	// Red for ERROR
	input = strings.ReplaceAll(input, "\x1b[1;31m", `<font style="color: #ef4444; display: inline;">`)
	// Reset
	input = strings.ReplaceAll(input, "\x1b[0m", `</font>`)

	return input
}

// BroadcastLog sends a log message to all connected clients for a key
func BroadcastLog(key string, logType string, message string) {
	// Trim whitespace from the message to avoid extra newlines
	message = strings.TrimSpace(message)
	// Console log to verify we're getting logs
	log.Debugf("[BROADCAST] Key %s [%s]: %s", key, logType, message)

	logStreamManager.mu.RLock()
	connections, exists := logStreamManager.clients[key]
	if !exists || len(connections) == 0 {
		logStreamManager.mu.RUnlock()
		log.Debugf("No active WebSocket connections for key %s", key)
		return
	}
	conns := make([]*websocket.Conn, len(connections))
	copy(conns, connections)
	logStreamManager.mu.RUnlock()

	// Create HTML for HTMX WebSocket extension
	// Send content that will be appended to #log-output-container
	// Using a wrapper with hx-swap-oob to ensure HTMX processes it correctly
	// HTML-escape the message first (ANSI codes don't contain HTML chars so they're unchanged)
	escapedMessage := html.EscapeString(message)
	// Then convert ANSI codes to HTML spans
	ansiConverted := ansiToHTML(escapedMessage)
	// Send a template fragment that HTMX can parse and inject
	htmlPayload := fmt.Sprintf(`<div hx-swap-oob="beforeend:#log-output-container" style="margin: 0; padding: 0;"><span style="white-space: nowrap;">%s</span></div>`, ansiConverted)
	payloadBytes := []byte(htmlPayload)

	log.Debugf("[WEBSOCKET] Broadcasting to %d clients for key %s: %s", len(conns), key, htmlPayload)

	// Track failed connections to clean up
	var failedConns []*websocket.Conn

	for i, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, payloadBytes); err != nil {
			log.Debugf("[WEBSOCKET] Failed to write to client %d for key %s: %v (will clean up)", i, key, err)
			failedConns = append(failedConns, conn)
		} else {
			log.Debugf("[WEBSOCKET] Successfully sent message to client %d for key %s", i, key)
		}
	}

	// Clean up failed connections
	if len(failedConns) > 0 {
		for _, failedConn := range failedConns {
			unregisterClient(key, failedConn)
		}
	}
}

func registerClient(key string, conn *websocket.Conn) {
	logStreamManager.mu.Lock()
	defer logStreamManager.mu.Unlock()

	if logStreamManager.clients[key] == nil {
		logStreamManager.clients[key] = make([]*websocket.Conn, 0)
	}
	logStreamManager.clients[key] = append(logStreamManager.clients[key], conn)
}

func unregisterClient(key string, conn *websocket.Conn) {
	logStreamManager.mu.Lock()
	defer logStreamManager.mu.Unlock()

	if connections, exists := logStreamManager.clients[key]; exists {
		for i, c := range connections {
			if c == conn {
				logStreamManager.clients[key] = append(connections[:i], connections[i+1:]...)
				conn.Close() // Close the connection
				break
			}
		}
		if len(logStreamManager.clients[key]) == 0 {
			delete(logStreamManager.clients, key)
		}
	}
}

// StartScriptExecution begins executing a script and streams logs. If createLog is true,
// an execution log will be created in the DB and returned. Returns error if the script is already running.
func StartScriptExecution(script *models.ScraperScript, variables map[string]string, createLog bool) (*models.ScraperExecutionLog, error) {
	// Prevent duplicate runs
	execContexts.mu.Lock()
	if _, exists := execContexts.funcs[script.ID]; exists {
		execContexts.mu.Unlock()
		return nil, fmt.Errorf("script already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	execContexts.funcs[script.ID] = cancel
	execContexts.mu.Unlock()

	// Notify that scraper has started
	if NotifyScraperStarted != nil {
		NotifyScraperStarted(script.ID, script.Name)
	}

	var execLog *models.ScraperExecutionLog
	if createLog {
		l, err := models.CreateScraperLog(script.ID, "running")
		if err != nil {
			log.Errorf("Failed to create execution log for script '%s': %v", script.Name, err)
		} else {
			execLog = l
			log.Infof("Created execution log ID=%d for script '%s'", l.ID, script.Name)
		}
	}

	// run in goroutine
	go func(s *models.ScraperScript, l *models.ScraperExecutionLog, vars map[string]string) {
		defer func() {
			execContexts.mu.Lock()
			delete(execContexts.funcs, s.ID)
			execContexts.mu.Unlock()

			// Notify that scraper has finished
			if NotifyScraperFinished != nil {
				NotifyScraperFinished(s.ID)
			}
		}()

		start := time.Now()
		BroadcastLog("scraper_"+strconv.FormatInt(s.ID, 10), "info", fmt.Sprintf("Script '%s' started executing...", s.Name))

		outputBuf := bytes.Buffer{}
		errMsg := ""

		if s.Language == "bash" {
			tmpFile, err := os.CreateTemp("", "scraper_*.sh")
			if err != nil {
				errMsg = fmt.Sprintf("Failed to create temporary script file: %v", err)
			} else {
				defer os.Remove(tmpFile.Name())

				// If shared script exists, create it and source it
				var sharedScriptPath string
				if s.SharedScript != nil && *s.SharedScript != "" {
					sharedTmpFile, err := os.CreateTemp("", "shared_*.sh")
					if err != nil {
						errMsg = fmt.Sprintf("Failed to create temporary shared script file: %v", err)
					} else {
						defer os.Remove(sharedTmpFile.Name())
						if _, err := sharedTmpFile.WriteString(*s.SharedScript); err != nil {
							errMsg = fmt.Sprintf("Failed to write shared script content: %v", err)
						}
						sharedTmpFile.Close()
						if err := os.Chmod(sharedTmpFile.Name(), 0755); err != nil {
							errMsg = fmt.Sprintf("Failed to make shared script executable: %v", err)
						} else {
							sharedScriptPath = sharedTmpFile.Name()
						}
					}
				}

				// Write the main script, sourcing the shared script if it exists
				scriptContent := s.Script
				if sharedScriptPath != "" {
					scriptContent = fmt.Sprintf("source %s\n%s", sharedScriptPath, s.Script)
				}

				if _, err := tmpFile.WriteString(scriptContent); err != nil {
					errMsg = fmt.Sprintf("Failed to write script content: %v", err)
				}
				tmpFile.Close()
				if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
					errMsg = fmt.Sprintf("Failed to make script executable: %v", err)
				} else {
					cmd := exec.CommandContext(ctx, "bash", "-u", tmpFile.Name())
					cmd.Env = os.Environ()
					for k, v := range vars {
						cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
					}

					// Combine stdout and stderr
					stdoutPipe, err := cmd.StdoutPipe()
					if err != nil {
						errMsg = fmt.Sprintf("Failed to create stdout pipe: %v", err)
					} else {
						cmd.Stderr = cmd.Stdout // Redirect stderr to stdout
						if err := cmd.Start(); err != nil {
							errMsg = fmt.Sprintf("Failed to start script: %v", err)
						} else {
							log.Debugf("Started bash script execution for script ID %d", s.ID)
							var wg sync.WaitGroup
							wg.Add(1)
							go func() {
								defer wg.Done()
								scanner := bufio.NewScanner(stdoutPipe)
								log.Debugf("[STDOUT] Starting to read combined output for script ID %d", s.ID)
								for scanner.Scan() {
									line := scanner.Text()
									log.Debugf("[STDOUT] Script %d: %s", s.ID, line)
									outputBuf.WriteString(line + "\n")
									BroadcastLog("scraper_"+strconv.FormatInt(s.ID, 10), "info", line)
								}
								if err := scanner.Err(); err != nil {
									log.Errorf("[STDOUT] Scanner error for script ID %d: %v", s.ID, err)
								}
								log.Debugf("[STDOUT] Finished reading combined output for script ID %d", s.ID)
							}()

							// Combined stdout and stderr, no separate stderr goroutine needed

							// Wait for command to complete
							if err := cmd.Wait(); err != nil {
								if outputBuf.Len() == 0 {
									errMsg = err.Error()
								} else {
									errMsg = strings.TrimSpace(outputBuf.String())
								}
							}

							// Wait for all output to be read from pipes
							wg.Wait()
						}
					}
				}
			}
		} else if s.Language == "python" {
			// Create temporary directory for virtual environment
			tmpDir, err := os.MkdirTemp("", "scraper_venv_")
			if err != nil {
				errMsg = fmt.Sprintf("Failed to create temporary directory: %v", err)
			} else {
				defer os.RemoveAll(tmpDir) // Clean up the entire directory

				venvPath := fmt.Sprintf("%s/venv", tmpDir)

				// Create virtual environment
				BroadcastLog("scraper_"+strconv.FormatInt(s.ID, 10), "info", "Creating Python virtual environment...")
				if err := exec.CommandContext(ctx, "python3", "-m", "venv", venvPath).Run(); err != nil {
					errMsg = fmt.Sprintf("Failed to create virtual environment: %v", err)
				} else {
					// Install packages if any
					if len(s.Packages) > 0 {
						BroadcastLog("scraper_"+strconv.FormatInt(s.ID, 10), "info", fmt.Sprintf("Installing packages: %s", strings.Join(s.Packages, ", ")))
						pipCmd := exec.CommandContext(ctx, fmt.Sprintf("%s/bin/pip", venvPath), "install")
						pipCmd.Args = append(pipCmd.Args, s.Packages...)
						pipCmd.Env = os.Environ()
						for k, v := range vars {
							pipCmd.Env = append(pipCmd.Env, fmt.Sprintf("%s=%s", k, v))
						}

						if output, err := pipCmd.CombinedOutput(); err != nil {
							errMsg = fmt.Sprintf("Failed to install packages: %v\nOutput: %s", err, string(output))
						} else {
							BroadcastLog("scraper_"+strconv.FormatInt(s.ID, 10), "info", "Packages installed successfully")
						}
					}

					if errMsg == "" {
						// Create temporary Python script file
						scriptFile := fmt.Sprintf("%s/script.py", tmpDir)
						if err := os.WriteFile(scriptFile, []byte(s.Script), 0644); err != nil {
							errMsg = fmt.Sprintf("Failed to write script content: %v", err)
						} else {
							// Run the Python script in the virtual environment
							cmd := exec.CommandContext(ctx, fmt.Sprintf("%s/bin/python", venvPath), scriptFile)
							cmd.Env = os.Environ()
							for k, v := range vars {
								cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
							}
							cmd.Dir = tmpDir // Set working directory to temp dir

							// Combine stdout and stderr
							stdoutPipe, err := cmd.StdoutPipe()
							if err != nil {
								errMsg = fmt.Sprintf("Failed to create stdout pipe: %v", err)
							} else {
								cmd.Stderr = cmd.Stdout // Redirect stderr to stdout
								if err := cmd.Start(); err != nil {
									errMsg = fmt.Sprintf("Failed to start script: %v", err)
								} else {
									log.Debugf("Started python script execution for script ID %d", s.ID)
									var wg sync.WaitGroup
									wg.Add(1)
									go func() {
										defer wg.Done()
										scanner := bufio.NewScanner(stdoutPipe)
										log.Debugf("[STDOUT] Starting to read combined output for script ID %d", s.ID)
										for scanner.Scan() {
											line := scanner.Text()
											log.Debugf("[STDOUT] Script %d: %s", s.ID, line)
											outputBuf.WriteString(line + "\n")
											BroadcastLog("scraper_"+strconv.FormatInt(s.ID, 10), "info", line)
										}
										if err := scanner.Err(); err != nil {
											log.Errorf("[STDOUT] Scanner error for script %d: %v", s.ID, err)
										}
										log.Debugf("[STDOUT] Finished reading stdout for script ID %d", s.ID)
									}()
									// Wait for command to complete
									if err := cmd.Wait(); err != nil {
										if outputBuf.Len() == 0 {
											errMsg = err.Error()
										} else {
											errMsg = strings.TrimSpace(outputBuf.String())
										}
									}

									// Wait for all output to be read from pipes
									wg.Wait()
								}
							}
						}
					}
				}
			}
		} else {
			errMsg = fmt.Sprintf("Unsupported language: %s (only 'bash' and 'python' are supported)", s.Language)
		}

		duration := time.Since(start)
		status := "success"
		if errMsg != "" {
			status = "error"
			BroadcastLog("scraper_"+strconv.FormatInt(s.ID, 10), "error", errMsg)
		}

		if l != nil {
			if err := models.UpdateScraperLogFinal(l.ID, status, outputBuf.String(), errMsg, duration.Milliseconds()); err != nil {
				log.Errorf("Failed to update execution log: %v", err)
			} else {
				log.Infof("Updated execution log ID=%d for script '%s' with status=%s", l.ID, s.Name, status)
			}
		}

		if err := models.UpdateScraperScriptLastRun(s.ID, outputBuf.String(), errMsg); err != nil {
			log.Errorf("Failed to update script last run: %v", err)
		}

		BroadcastLog("scraper_"+strconv.FormatInt(s.ID, 10), "info", fmt.Sprintf("Script execution completed in %s", duration.String()))
		log.Infof("Script '%s' (ID=%d) finished with status: %s", s.Name, s.ID, status)
	}(script, execLog, variables)

	return execLog, nil
}

// CancelScriptExecution cancels a running script by ID
func CancelScriptExecution(scriptID int64) error {
	execContexts.mu.RLock()
	cancel, exists := execContexts.funcs[scriptID]
	execContexts.mu.RUnlock()
	if !exists {
		return fmt.Errorf("no running script with id %d", scriptID)
	}
	cancel()
	BroadcastLog("scraper_"+strconv.FormatInt(scriptID, 10), "info", "Script execution cancelled by user")
	return nil
}

// InitializeScraperScheduler initializes the scraper scheduler
func InitializeScraperScheduler() {
	scraperExecuteFunc = func(script *models.ScraperScript) error {
		log.Infof("Starting scheduled script '%s' (ID=%d)", script.Name, script.ID)
		if _, err := StartScriptExecution(script, script.Variables, true); err != nil {
			log.Infof("Scheduled run skipped: script '%s' (ID=%d) is already running: %v", script.Name, script.ID, err)
			return nil
		}
		return nil
	}
	scraperMutex.Lock()
	defer scraperMutex.Unlock()

	if scraperScheduler != nil {
		scraperScheduler.Stop()
	}

	scraperScheduler = NewCronScheduler()
	scraperScheduler.Start()

	// Load and register all enabled scraper scripts
	scripts, err := models.ListScraperScripts(true)
	if err != nil {
		log.Errorf("Failed to load scraper scripts: %v", err)
		return
	}

	for _, script := range scripts {
		// create a local copy to avoid closing over the loop variable's address
		s := script
		if err := RegisterScraperScript(&s); err != nil {
			log.Errorf("Failed to register scraper script '%s': %v", script.Name, err)
		}
	}

	log.Debugf("Scraper scheduler initialized with %d scripts", len(scripts))
}

// ReloadScraperScheduler reloads the scraper scheduler
func ReloadScraperScheduler() {
	log.Info("Reloading scraper scheduler")
	InitializeScraperScheduler()
}

// StopScraperScheduler stops the scraper scheduler
func StopScraperScheduler() {
	scraperMutex.Lock()
	defer scraperMutex.Unlock()

	if scraperScheduler != nil {
		log.Info("Stopping scraper scheduler")
		scraperScheduler.Stop()
		scraperScheduler = nil
	}
}

// InitializeSubscriptionScheduler initializes the subscription expiry scheduler
func InitializeSubscriptionScheduler() {
	subscriptionMutex.Lock()
	defer subscriptionMutex.Unlock()

	if subscriptionScheduler != nil {
		subscriptionScheduler.Stop()
	}

	subscriptionScheduler = NewCronScheduler()
	subscriptionScheduler.Start()

	// Run subscription expiry check every hour
	job := &SubscriptionExpiryJob{}
	if err := subscriptionScheduler.AddJob(job.Name(), "0 * * * *", job); err != nil {
		log.Errorf("Failed to register subscription expiry job: %v", err)
	} else {
		log.Info("Subscription expiry scheduler initialized - checking every hour")
	}
}

// ReloadSubscriptionScheduler reloads the subscription scheduler
func ReloadSubscriptionScheduler() {
	log.Info("Reloading subscription scheduler")
	InitializeSubscriptionScheduler()
}

// StopSubscriptionScheduler stops the subscription scheduler
func StopSubscriptionScheduler() {
	subscriptionMutex.Lock()
	defer subscriptionMutex.Unlock()

	if subscriptionScheduler != nil {
		log.Info("Stopping subscription scheduler")
		subscriptionScheduler.Stop()
		subscriptionScheduler = nil
	}
}
