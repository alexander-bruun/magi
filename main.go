package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/alexander-bruun/magi/cmd"
	"github.com/alexander-bruun/magi/filestore"
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

	// Cache backend configuration
	var backend string
	var localPath string
	var sftpHost string
	var sftpPort int
	var sftpUsername string
	var sftpPassword string
	var sftpKeyFile string
	var sftpBasePath string
	var s3Bucket string
	var s3Region string
	var s3Endpoint string
	var s3BasePath string

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

			// Configure cache backend with priority hierarchy: CLI flags > Env vars > Default
			cacheConfig := &filestore.CacheConfig{
				BackendType: getConfigValue(backend, os.Getenv("MAGI_CACHE_BACKEND"), "local"),
			}

			switch cacheConfig.BackendType {
			case "local":
				cacheConfig.LocalBasePath = getConfigValue(localPath, os.Getenv("MAGI_CACHE_LOCAL_PATH"), cacheDirectory)
			case "sftp":
				cacheConfig.SFTPHost = getConfigValue(sftpHost, os.Getenv("MAGI_CACHE_SFTP_HOST"), "")
				cacheConfig.SFTPPort = getConfigIntValue(sftpPort, getEnvIntOrDefault("MAGI_CACHE_SFTP_PORT", 22), 22)
				cacheConfig.SFTPUsername = getConfigValue(sftpUsername, os.Getenv("MAGI_CACHE_SFTP_USERNAME"), "")
				cacheConfig.SFTPPassword = getConfigValue(sftpPassword, os.Getenv("MAGI_CACHE_SFTP_PASSWORD"), "")
				cacheConfig.SFTPKeyFile = getConfigValue(sftpKeyFile, os.Getenv("MAGI_CACHE_SFTP_KEY_FILE"), "")
				cacheConfig.SFTPBasePath = getConfigValue(sftpBasePath, os.Getenv("MAGI_CACHE_SFTP_BASE_PATH"), "")
			case "s3":
				cacheConfig.S3Bucket = getConfigValue(s3Bucket, os.Getenv("MAGI_CACHE_S3_BUCKET"), "")
				cacheConfig.S3Region = getConfigValue(s3Region, os.Getenv("MAGI_CACHE_S3_REGION"), "")
				cacheConfig.S3Endpoint = getConfigValue(s3Endpoint, os.Getenv("MAGI_CACHE_S3_ENDPOINT"), "")
				cacheConfig.S3BasePath = getConfigValue(s3BasePath, os.Getenv("MAGI_CACHE_S3_BASE_PATH"), "")
			}

			// Validate cache configuration
			if err := cacheConfig.Validate(); err != nil {
				log.Errorf("Invalid cache configuration: %v", err)
				return
			}

			// Create cache backend
			cacheBackendInstance, err := cacheConfig.CreateBackend()
			if err != nil {
				log.Errorf("Failed to create cache backend: %v", err)
				return
			}

			log.Infof("Using cache backend: %s", cacheConfig.BackendType)

			log.Debugf("Using '%s/magi.db,-shm,-wal' as the database location", dataDirectory)
			log.Debugf("Using '%s/...' as the image caching location", cacheDirectory)
			log.Debugf("Using '%s/...' as the backup location", backupDirectory)

			// Initialize console log streaming for admin panel
			utils.InitializeConsoleLogger()

			// Initialize database connection
			err = models.Initialize(dataDirectory)
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

			// Custom config optimized for 10k concurrent users
			app := fiber.New(fiber.Config{
				Prefork:          false, // Use single process (no prefork with SQLite)
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
			go handlers.Initialize(app, cacheBackendInstance, backupDirectory, port)

			// Start Indexer in its own goroutine
			libraries, err := models.GetLibraries()
			if err != nil {
				log.Warnf("Failed to get libraries: %v", err)
				return
			}
			go scheduler.InitializeIndexer(cacheDirectory, libraries, cacheBackendInstance)
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

	// Cache backend flags
	rootCmd.PersistentFlags().StringVar(&backend, "backend", os.Getenv("MAGI_CACHE_BACKEND"), "Cache backend type (local, sftp, s3)")
	rootCmd.PersistentFlags().StringVar(&localPath, "local-path", os.Getenv("MAGI_CACHE_LOCAL_PATH"), "Local cache directory path")

	// SFTP cache flags
	rootCmd.PersistentFlags().StringVar(&sftpHost, "sftp-host", os.Getenv("MAGI_CACHE_SFTP_HOST"), "SFTP cache host")
	rootCmd.PersistentFlags().IntVar(&sftpPort, "sftp-port", getEnvIntOrDefault("MAGI_CACHE_SFTP_PORT", 22), "SFTP cache port")
	rootCmd.PersistentFlags().StringVar(&sftpUsername, "sftp-username", os.Getenv("MAGI_CACHE_SFTP_USERNAME"), "SFTP cache username")
	rootCmd.PersistentFlags().StringVar(&sftpPassword, "sftp-password", os.Getenv("MAGI_CACHE_SFTP_PASSWORD"), "SFTP cache password")
	rootCmd.PersistentFlags().StringVar(&sftpKeyFile, "sftp-key-file", os.Getenv("MAGI_CACHE_SFTP_KEY_FILE"), "SFTP cache private key file")
	rootCmd.PersistentFlags().StringVar(&sftpBasePath, "sftp-base-path", os.Getenv("MAGI_CACHE_SFTP_BASE_PATH"), "SFTP cache base path")

	// S3 cache flags
	rootCmd.PersistentFlags().StringVar(&s3Bucket, "s3-bucket", os.Getenv("MAGI_CACHE_S3_BUCKET"), "S3 cache bucket")
	rootCmd.PersistentFlags().StringVar(&s3Region, "s3-region", os.Getenv("MAGI_CACHE_S3_REGION"), "S3 cache region")
	rootCmd.PersistentFlags().StringVar(&s3Endpoint, "s3-endpoint", os.Getenv("MAGI_CACHE_S3_ENDPOINT"), "S3 cache endpoint (for S3-compatible services)")
	rootCmd.PersistentFlags().StringVar(&s3BasePath, "s3-base-path", os.Getenv("MAGI_CACHE_S3_BASE_PATH"), "S3 cache base path")

	// Add commands
	rootCmd.AddCommand(cmd.NewVersionCmd(Version))
	rootCmd.AddCommand(cmd.NewMigrateCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewBackendMigrateCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewUserCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewLibraryCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewSeriesCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewBackupCmd(&dataDirectory, &backupDirectory))
	rootCmd.AddCommand(cmd.NewHighlightsCmd(&dataDirectory))
	rootCmd.AddCommand(cmd.NewMaintenanceCmd(&dataDirectory))

	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
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

// getConfigIntValue returns the first non-zero value from CLI flag, env var, or default
func getConfigIntValue(cliFlag, envVar, defaultValue int) int {
	if cliFlag != 0 {
		return cliFlag
	}
	if envVar != 0 {
		return envVar
	}
	return defaultValue
}
