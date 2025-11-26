package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"embed"
	"net/http"

	"github.com/alexander-bruun/magi/executor"
	"github.com/alexander-bruun/magi/handlers"
	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/models"
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

			// Ensure the directories exist
			if err := os.MkdirAll(cacheDirectory, os.ModePerm); err != nil {
				log.Errorf("Failed to create cache directory: %s", err)
				return
			}

			log.Debugf("Using '%s/magi.db,-shm,-wal' as the database location", dataDirectory)
			log.Debugf("Using '%s/...' as the image caching location", cacheDirectory)

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
			go handlers.Initialize(app, cacheDirectory, port)

			// Start Indexer in its own goroutine
			libraries, err := models.GetLibraries()
			if err != nil {
				log.Warnf("Failed to get libraries: %v", err)
				return
			}
			go indexer.Initialize(cacheDirectory, libraries)

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			log.Info("Magi started successfully. Press Ctrl+C to stop.")

			// Wait for shutdown signal
			<-sigChan
			log.Info("Received shutdown signal, stopping services...")

			// Stop all background services
			indexer.StopAllIndexers()
			handlers.StopTokenCleanup()
			executor.StopScraperScheduler()

			log.Info("Shutdown complete.")
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&dataDirectory, "data-directory", defaultDataDirectory, "Path to the data directory")
	rootCmd.PersistentFlags().StringVar(&cacheDirectory, "cache-directory", os.Getenv("MAGI_CACHE_DIR"), "Path to the cache directory")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", os.Getenv("LOG_LEVEL"), "Set the log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&port, "port", os.Getenv("PORT"), "Port to run the server on")

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			log.Infof("Version: %s", Version)
		},
	}

	var migrateCmd = &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
	}

	var migrateUpCmd = &cobra.Command{
		Use:   "up [version|all]",
		Short: "Run migrations up to a specific version or all pending migrations",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var version int
			var err error
			if args[0] == "all" {
				// Apply all pending migrations
				err = models.InitializeWithMigration(dataDirectory, true)
				if err != nil {
					log.Errorf("Failed to apply migrations: %v", err)
					os.Exit(1)
				}
				log.Info("All pending migrations applied successfully")
				return
			} else {
				version, err = strconv.Atoi(args[0])
				if err != nil {
					log.Errorf("Invalid version number: %s", args[0])
					os.Exit(1)
				}
			}

			// Initialize database without auto-migrations
			err = models.InitializeWithMigration(dataDirectory, false)
			if err != nil {
				log.Errorf("Failed to connect to database: %v", err)
				os.Exit(1)
			}
			defer models.Close()

			err = models.MigrateUp("migrations", version)
			if err != nil {
				log.Errorf("Migration failed: %v", err)
				os.Exit(1)
			}

			log.Infof("Migration up %d completed successfully", version)
		},
	}

	var migrateDownCmd = &cobra.Command{
		Use:   "down [version|all]",
		Short: "Rollback migrations down to a specific version or rollback all migrations",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var version int
			var err error
			if args[0] == "all" {
				// Rollback all migrations
				err = models.InitializeWithMigration(dataDirectory, false)
				if err != nil {
					log.Errorf("Failed to connect to database: %v", err)
					os.Exit(1)
				}
				defer models.Close()

				// Get all applied migrations and rollback them in reverse order
				// For simplicity, rollback from highest to lowest
				for v := 17; v >= 1; v-- {
					err = models.MigrateDown("migrations", v)
					if err != nil {
						log.Errorf("Failed to rollback migration %d: %v", v, err)
						os.Exit(1)
					}
				}
				log.Info("All migrations rolled back successfully")
				return
			} else {
				version, err = strconv.Atoi(args[0])
				if err != nil {
					log.Errorf("Invalid version number: %s", args[0])
					os.Exit(1)
				}
			}

			// Initialize database without auto-migrations
			err = models.InitializeWithMigration(dataDirectory, false)
			if err != nil {
				log.Errorf("Failed to connect to database: %v", err)
				os.Exit(1)
			}
			defer models.Close()

			err = models.MigrateDown("migrations", version)
			if err != nil {
				log.Errorf("Migration failed: %v", err)
				os.Exit(1)
			}

			log.Infof("Migration down %d completed successfully", version)
		},
	}

	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(migrateCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
