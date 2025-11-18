package executor

import (
    "bufio"
    "bytes"
    "context"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "sync"
    "time"

    "github.com/gofiber/fiber/v2/log"
    "github.com/gofiber/websocket/v2"

    "github.com/alexander-bruun/magi/models"
)

// LogStreamManager manages WebSocket connections for log streaming
type LogStreamManager struct {
    clients map[int64][]*websocket.Conn // scriptID -> list of connections
    mu      sync.RWMutex
}

var logStreamManager = &LogStreamManager{
    clients: make(map[int64][]*websocket.Conn),
}

// Execution contexts for running scripts (to allow cancellation)
var execContexts = struct {
    mu    sync.RWMutex
    funcs map[int64]context.CancelFunc
}{
    funcs: make(map[int64]context.CancelFunc),
}

// HandleLogsWebSocket establishes a WebSocket connection for streaming logs
func HandleLogsWebSocket(c *websocket.Conn, scriptID int64) {
    // Register the connection
    registerClient(scriptID, c)
    defer func() {
        unregisterClient(scriptID, c)
        log.Debugf("WebSocket client disconnected for script %d", scriptID)
    }()

    log.Debugf("WebSocket client connected for script %d", scriptID)

    // Set up pong handler to detect connection health
    c.SetPongHandler(func(string) error {
        log.Debugf("Received pong from client for script %d", scriptID)
        return nil
    })

    // Keep connection alive and wait for close
    for {
        _, _, err := c.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
                log.Debugf("WebSocket closed for script %d: %v", scriptID, err)
            }
            break
        }
    }
}

// BroadcastLog sends a log message to all connected clients for a script
func BroadcastLog(scriptID int64, logType string, message string) {
    // Console log to verify we're getting logs
    log.Debugf("[BROADCAST] Script %d [%s]: %s", scriptID, logType, message)
    
    logStreamManager.mu.RLock()
    connections, exists := logStreamManager.clients[scriptID]
    if !exists || len(connections) == 0 {
        logStreamManager.mu.RUnlock()
        log.Debugf("No active WebSocket connections for script %d", scriptID)
        return
    }
    conns := make([]*websocket.Conn, len(connections))
    copy(conns, connections)
    logStreamManager.mu.RUnlock()

    // Simple JSON construction to avoid additional imports
    // marshal manually
    b := bytes.Buffer{}
    b.WriteString("{")
    b.WriteString(fmt.Sprintf("\"type\":\"%s\",", strings.ReplaceAll(logType, "\"", "\\\"")))
    b.WriteString(fmt.Sprintf("\"message\":\"%s\"", strings.ReplaceAll(message, "\"", "\\\"")))
    b.WriteString("}")
    payloadBytes := b.Bytes()

    log.Debugf("[WEBSOCKET] Broadcasting to %d clients for script %d: %s", len(conns), scriptID, string(payloadBytes))
    
    // Track failed connections to clean up
    var failedConns []*websocket.Conn
    
    for i, conn := range conns {
        if err := conn.WriteMessage(websocket.TextMessage, payloadBytes); err != nil {
            log.Debugf("[WEBSOCKET] Failed to write to client %d for script %d: %v (will clean up)", i, scriptID, err)
            failedConns = append(failedConns, conn)
        } else {
            log.Debugf("[WEBSOCKET] Successfully sent message to client %d for script %d", i, scriptID)
        }
    }
    
    // Clean up failed connections
    if len(failedConns) > 0 {
        logStreamManager.mu.Lock()
        for _, failedConn := range failedConns {
            if connections, exists := logStreamManager.clients[scriptID]; exists {
                for i, c := range connections {
                    if c == failedConn {
                        logStreamManager.clients[scriptID] = append(connections[:i], connections[i+1:]...)
                        log.Debugf("[WEBSOCKET] Removed stale connection for script %d", scriptID)
                        break
                    }
                }
                if len(logStreamManager.clients[scriptID]) == 0 {
                    delete(logStreamManager.clients, scriptID)
                    log.Debugf("[WEBSOCKET] Removed all connections for script %d", scriptID)
                }
            }
        }
        logStreamManager.mu.Unlock()
    }
}

func registerClient(scriptID int64, conn *websocket.Conn) {
    logStreamManager.mu.Lock()
    defer logStreamManager.mu.Unlock()

    if logStreamManager.clients[scriptID] == nil {
        logStreamManager.clients[scriptID] = make([]*websocket.Conn, 0)
    }
    logStreamManager.clients[scriptID] = append(logStreamManager.clients[scriptID], conn)
}

func unregisterClient(scriptID int64, conn *websocket.Conn) {
    logStreamManager.mu.Lock()
    defer logStreamManager.mu.Unlock()

    if connections, exists := logStreamManager.clients[scriptID]; exists {
        for i, c := range connections {
            if c == conn {
                logStreamManager.clients[scriptID] = append(connections[:i], connections[i+1:]...)
                break
            }
        }
        if len(logStreamManager.clients[scriptID]) == 0 {
            delete(logStreamManager.clients, scriptID)
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
        }()

        start := time.Now()
        BroadcastLog(s.ID, "info", fmt.Sprintf("Script '%s' started executing...", s.Name))

        outputBuf := bytes.Buffer{}
        errMsg := ""

        if s.Language == "bash" {
            tmpFile, err := os.CreateTemp("", "scraper_*.sh")
            if err != nil {
                errMsg = fmt.Sprintf("Failed to create temporary script file: %v", err)
            } else {
                defer os.Remove(tmpFile.Name())
                if _, err := tmpFile.WriteString(s.Script); err != nil {
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

                    stdoutPipe, err := cmd.StdoutPipe()
                    if err != nil {
                        errMsg = fmt.Sprintf("Failed to create stdout pipe: %v", err)
                    } else {
                        stderrPipe, err := cmd.StderrPipe()
                        if err != nil {
                            errMsg = fmt.Sprintf("Failed to create stderr pipe: %v", err)
                        } else if err := cmd.Start(); err != nil {
                            errMsg = fmt.Sprintf("Failed to start script: %v", err)
                        } else {
                            log.Debugf("Started bash script execution for script ID %d", s.ID)
                            var wg sync.WaitGroup
                            wg.Add(1)
                            go func() {
                                defer wg.Done()
                                scanner := bufio.NewScanner(stdoutPipe)
                                log.Debugf("[STDOUT] Starting to read stdout for script ID %d", s.ID)
                                for scanner.Scan() {
                                    line := scanner.Text()
                                    log.Debugf("[STDOUT] Script %d: %s", s.ID, line)
                                    outputBuf.WriteString(line + "\n")
                                    BroadcastLog(s.ID, "info", line)
                                }
                                if err := scanner.Err(); err != nil {
                                    log.Errorf("[STDOUT] Scanner error for script %d: %v", s.ID, err)
                                }
                                log.Debugf("[STDOUT] Finished reading stdout for script ID %d", s.ID)
                            }()

                            wg.Add(1)
                            go func() {
                                defer wg.Done()
                                scanner := bufio.NewScanner(stderrPipe)
                                log.Debugf("[STDERR] Starting to read stderr for script ID %d", s.ID)
                                for scanner.Scan() {
                                    line := scanner.Text()
                                    log.Debugf("[STDERR] Script %d: %s", s.ID, line)
                                    outputBuf.WriteString(line + "\n")
                                    BroadcastLog(s.ID, "error", line)
                                }
                                if err := scanner.Err(); err != nil {
                                    log.Errorf("[STDERR] Scanner error for script %d: %v", s.ID, err)
                                }
                                log.Debugf("[STDERR] Finished reading stderr for script ID %d", s.ID)
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
        } else {
            errMsg = fmt.Sprintf("Unsupported language: %s (only 'bash' is supported)", s.Language)
        }

        duration := time.Since(start)
        status := "success"
        if errMsg != "" {
            status = "error"
            BroadcastLog(s.ID, "error", errMsg)
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

        BroadcastLog(s.ID, "info", fmt.Sprintf("Script execution completed in %s", duration.String()))
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
    BroadcastLog(scriptID, "info", "Script execution cancelled by user")
    return nil
}
