package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"embed"
	"net/http"

	"github.com/alexander-bruun/magi/cmd"
	"github.com/alexander-bruun/magi/handlers"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/scheduler"
	"github.com/alexander-bruun/magi/utils"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	html "github.com/gofiber/template/html/v2"
	"github.com/spf13/cobra"
)

var Version = "develop"

//go:embed views/*
var viewsfs embed.FS

//go:embed assets/*
var assetsfs embed.FS

func main() {
	var dataDirectory string
	var logLevel string
	var cacheDirectory string
	var backupDirectory string
	var port string

	var defaultDataDirectory string

	// Check for environment variable override
	if envDataDir := os.Getenv("MAGI_DATA_DIR"); envDataDir != "" {
		defaultDataDirectory = envDataDir
	} else {
		// OS-specific defaults
		switch runtime.GOOS {
		case "windows":
			defaultDataDirectory = filepath.Join(os.Getenv("LOCALAPPDATA"), "magi")
		case "darwin":
			// macOS
			defaultDataDirectory = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "magi")
		case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
			defaultDataDirectory = filepath.Join(os.Getenv("HOME"), "magi")
		case "plan9":
			defaultDataDirectory = filepath.Join(os.Getenv("home"), "magi")
		case "solaris":
			defaultDataDirectory = filepath.Join(os.Getenv("HOME"), "magi")
		default:
			// Fallback for unknown OS
			defaultDataDirectory = filepath.Join(os.Getenv("HOME"), "magi")
		}
	}

	var rootCmd = &cobra.Command{
		Use:   "magi",
		Short: "Magi - A manga reader application",
		Long:  `Magi is a web-based manga reader application.`,
		Run: func(cmd *cobra.Command, args []string) {
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

			log.Info("Starting Magi!")

			// Determine cache directory
			if cacheDirectory == "" {
				cacheDirectory = filepath.Join(dataDirectory, "cache")
			}

			// Determine backup directory
			if backupDirectory == "" {
				backupDirectory = filepath.Join(dataDirectory, "backups")
			}

			// Ensure the directories exist
			if err := os.MkdirAll(cacheDirectory, os.ModePerm); err != nil {
				log.Errorf("Failed to create cache directory: %s", err)
				return
			}
			if err := os.MkdirAll(backupDirectory, os.ModePerm); err != nil {
				log.Errorf("Failed to create backup directory: %s", err)
				return
			}

			log.Debugf("Using '%s/magi.db,-shm,-wal' as the database location", dataDirectory)
			log.Debugf("Using '%s/...' as the image caching location", cacheDirectory)
			log.Debugf("Using '%s/...' as the backup location", backupDirectory)

			// Initialize console log streaming for admin panel
			utils.InitializeConsoleLogger()

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

			// Abort any orphaned "running" logs from previous application run
			if err := models.AbortOrphanedRunningLogs(); err != nil {
				log.Warnf("Failed to abort orphaned running logs: %v", err)
			}

			// Create a new engine
			engine := html.NewFileSystem(http.FS(viewsfs), ".html")

			// Custom config
			app := fiber.New(fiber.Config{
				Prefork:       false,
				CaseSensitive: true,
				StrictRouting: true,
				ServerHeader:  "Magi",
				AppName:       fmt.Sprintf("Magi %s", Version),
				Views:         engine,
				ViewsLayout:   "base",
			})

			// Start API in its own goroutine
			go handlers.Initialize(app, cacheDirectory, backupDirectory, port)

			// Start Indexer in its own goroutine
			libraries, err := models.GetLibraries()
			if err != nil {
				log.Warnf("Failed to get libraries: %v", err)
				return
			}
			go scheduler.InitializeIndexer(cacheDirectory, libraries)
			go scheduler.InitializeScraperScheduler()

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			log.Info("Magi started successfully. Press Ctrl+C to stop.")

			// Wait for shutdown signal
			select {
			case <-sigChan:
				log.Info("Received shutdown signal, stopping services...")
			case <-handlers.GetShutdownChan():
				log.Info("Received internal shutdown request, stopping services...")
			}

			// Stop all background services
			scheduler.StopAllIndexers()
			handlers.StopTokenCleanup()
			scheduler.StopScraperScheduler()

			log.Info("Shutdown complete.")
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&dataDirectory, "data-directory", defaultDataDirectory, "Path to the data directory")
	rootCmd.PersistentFlags().StringVar(&cacheDirectory, "cache-directory", os.Getenv("MAGI_CACHE_DIR"), "Path to the cache directory")
	rootCmd.PersistentFlags().StringVar(&backupDirectory, "backup-directory", os.Getenv("MAGI_BACKUP_DIR"), "Path to the backup directory")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", os.Getenv("LOG_LEVEL"), "Set the log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&port, "port", os.Getenv("PORT"), "Port to run the server on")

	// Add commands
	rootCmd.AddCommand(cmd.NewVersionCmd(Version))
	rootCmd.AddCommand(cmd.NewMigrateCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewUserCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewBackupCmd(&dataDirectory, &backupDirectory))

	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
