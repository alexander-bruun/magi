package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/alexander-bruun/magi/cmd"
	"github.com/alexander-bruun/magi/handlers"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/scheduler"
	"github.com/alexander-bruun/magi/utils/embedded"
	"github.com/alexander-bruun/magi/utils/files"
	"github.com/alexander-bruun/magi/utils/store"
	"github.com/alexander-bruun/magi/utils/text"

	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	html "github.com/gofiber/template/html/v3"
	"github.com/spf13/cobra"
)

var Version = "develop"

//go:embed views/*
var viewsfs embed.FS

//go:embed assets/*
var assetsfs embed.FS

//go:embed migrations
var migrationsfs embed.FS

func main() {
	// Register all embedded filesystems in the central package
	embedded.Init(viewsfs, assetsfs, migrationsfs)

	var dataDirectory string
	var logLevel string
	var port string

	// Data backend configuration
	var localPath string

	var defaultDataDirectory string

	// Check for environment variable override
	if envDataDir := os.Getenv("MAGI_DATA_DIR"); envDataDir != "" {
		defaultDataDirectory = envDataDir
	} else {
		// OS-specific defaults
		switch runtime.GOOS {
		case "windows":
			defaultDataDirectory = filepath.Join(os.Getenv("LOCALAPPDATA"), "magi")
		case "linux":
			defaultDataDirectory = filepath.Join(os.Getenv("HOME"), ".magi")
		default:
			// Fallback for unknown OS
			defaultDataDirectory = filepath.Join(os.Getenv("HOME"), ".magi")
		}
	}

	// Ensure the default data directory is absolute
	if !filepath.IsAbs(defaultDataDirectory) {
		if abs, err := filepath.Abs(defaultDataDirectory); err == nil {
			defaultDataDirectory = abs
		}
	}

	var rootCmd = &cobra.Command{
		Use:   "magi",
		Short: "Magi - A manga reader application",
		Long:  `Magi is a web-based manga reader application.`,
		Run: func(cmd *cobra.Command, args []string) {
			var backupDirectory string
			// Set log level
			if logLevel == "" {
				logLevel = "info"
			}
			switch logLevel {
			case "debug":
				log.SetLevel(log.LevelDebug)
			case "warn":
				log.SetLevel(log.LevelWarn)
			case "error":
				log.SetLevel(log.LevelError)
			default:
				log.SetLevel(log.LevelInfo)
			}

			// Initialize console log streaming for admin panel
			text.InitializeConsoleLogger()

			if os.Getenv("FIBER_PREFORK_CHILD") == "" {
				log.Info("Starting Magi!")
			}

			// Ensure dataDirectory is absolute
			if !filepath.IsAbs(dataDirectory) {
				if abs, err := filepath.Abs(dataDirectory); err == nil {
					dataDirectory = abs
				}
			}

			// Set the data directory for utility functions
			files.SetDataDirectory(dataDirectory)

			// Determine backup directory
			backupDirectory = filepath.Join(dataDirectory, "backups")

			// Ensure the directories exist
			if err := os.MkdirAll(dataDirectory, os.ModePerm); err != nil {
				log.Errorf("Failed to create data directory: %s", err)
				return
			}
			if err := os.MkdirAll(backupDirectory, os.ModePerm); err != nil {
				log.Errorf("Failed to create backup directory: %s", err)
				return
			}

			// Configure local data backend with priority hierarchy: CLI flags > Env vars > Default
			dataBasePath := getConfigValue(localPath, os.Getenv("MAGI_DATA_LOCAL_PATH"), dataDirectory)
			if dataBasePath == "" {
				log.Error("Data directory path is required")
				return
			}

			// Create file store
			fileStore := store.NewFileStore(dataBasePath)

			if os.Getenv("FIBER_PREFORK_CHILD") == "" {
				log.Info("Using local data backend")

				log.Debugf("Using '%s/magi.db,-shm,-wal' as the database location", dataDirectory)
				log.Debugf("Using '%s/...' as the image caching location", dataDirectory)
				log.Debugf("Using '%s/...' as the backup location", backupDirectory)
			}

			// Initialize console log streaming for admin panel
			text.InitializeConsoleLogger()

			// Initialize database connection
			err := models.Initialize(dataDirectory)
			if err != nil {
				log.Errorf("Failed to connect to database: %v", err)
				return
			}
			defer func() {
				if err := models.Close(); err != nil {
					log.Errorf("Failed to close database: %v", err)
				}
			}()
			defer handlers.StopJobStatusManager()

			// Generate ImageAccessSecret if not set
			cfg, err := models.GetAppConfig()
			if err != nil {
				log.Errorf("Failed to get app config: %v", err)
				return
			}
			if cfg.ImageAccessSecret == "" {
				bytes := make([]byte, 32)
				if _, err := rand.Read(bytes); err != nil {
					log.Errorf("Failed to generate random secret: %v", err)
					return
				}
				cfg.ImageAccessSecret = hex.EncodeToString(bytes)
				_, err = models.UpdateImageAccessSecret(cfg.ImageAccessSecret)
				if err != nil {
					log.Errorf("Failed to update image access secret: %v", err)
					return
				}
				log.Info("Generated new ImageAccessSecret")
			}

			// Initialize slug key for image URL generation (shared across prefork via DB)
			files.SetSlugKey(cfg.ImageAccessSecret)

			// Abort any orphaned "running" logs from previous application run
			if err := models.AbortOrphanedRunningLogs(); err != nil {
				log.Warnf("Failed to abort orphaned running logs: %v", err)
			}

			// Create a new engine
			engine := html.NewFileSystem(http.FS(embedded.Views), ".html")

			// Custom config optimized for 10k concurrent users
			app := fiber.New(fiber.Config{
				CaseSensitive:    true,
				StrictRouting:    true,
				ServerHeader:     "Magi",
				AppName:          fmt.Sprintf("Magi %s", Version),
				Views:            engine,
				ViewsLayout:      "base",
				BodyLimit:        50 * 1024 * 1024,
				Concurrency:      262144,
				ReadBufferSize:   16 * 1024,
				WriteBufferSize:  16 * 1024,
				ReadTimeout:      30 * time.Second,
				WriteTimeout:     30 * time.Second,
				IdleTimeout:      5 * time.Minute,
				DisableKeepalive: false,
			})

			// Start API in its own goroutine
			go handlers.Initialize(app, fileStore, backupDirectory, port)

			// Start Indexer and Scheduler only in master process
			if os.Getenv("FIBER_PREFORK_CHILD") == "" {
				libraries, err := models.GetLibraries()
				if err != nil {
					log.Warnf("Failed to get libraries: %v", err)
					return
				}
				scheduler.InitializeIndexer(dataDirectory, libraries, fileStore)
				scheduler.InitializeScraperScheduler()
				scheduler.InitializeSubscriptionScheduler()
			}

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			if os.Getenv("FIBER_PREFORK_CHILD") == "" {
				log.Info("Magi started successfully. Press Ctrl+C to stop.")
			}

			// Wait for shutdown signal
			select {
			case <-sigChan:
				log.Info("Received shutdown signal, stopping services...")
			case <-handlers.GetShutdownChan():
				log.Info("Received internal shutdown request, stopping services...")
			}

			// Create a context with timeout for graceful shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			// Stop all background services with timeout
			done := make(chan struct{})
			go func() {
				defer close(done)
				scheduler.StopAllIndexers()
				scheduler.StopScraperScheduler()
				scheduler.StopSubscriptionScheduler()
			}()

			select {
			case <-done:
				log.Info("Shutdown complete.")
			case <-shutdownCtx.Done():
				log.Warn("Shutdown timed out, forcing exit.")
			}
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&dataDirectory, "data-directory", defaultDataDirectory, "Path to the data directory")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", os.Getenv("LOG_LEVEL"), "Set the log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&port, "port", os.Getenv("PORT"), "Port to run the server on")

	// Data backend flags
	rootCmd.PersistentFlags().StringVar(&localPath, "local-path", os.Getenv("MAGI_DATA_LOCAL_PATH"), "Local data directory path")

	// Add commands
	rootCmd.AddCommand(cmd.NewVersionCmd(Version))
	rootCmd.AddCommand(cmd.NewMigrateCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewUserCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewLibraryCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewSeriesCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewBackupCmd(&dataDirectory, &dataDirectory))
	rootCmd.AddCommand(cmd.NewHighlightsCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewMaintenanceCmd(&dataDirectory))

	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

// getConfigValue returns the first non-empty value from CLI flag, env var, or default
func getConfigValue(cliFlag, envVar, defaultValue string) string {
	if cliFlag != "" {
		return cliFlag
	}
	if envVar != "" {
		return envVar
	}
	return defaultValue
}
