package main

import (
	// _ "net/http/pprof" // Import for side-effect of registering pprof handlers

	"embed"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/alexander-bruun/magi/handlers"
	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/template/html/v2"
)

var Version = "develop"

// //go:embed migrations/*.sql
// var MigrationsDirectory embed.FS

// //go:embed views/*.go
// var ViewsDirectory embed.FS

// //go:embed assets/*
// var AssetsDirectory embed.FS

//go:embed views/*
var viewsfs embed.FS

//go:embed assets/*
var assetsfs embed.FS

var dataDirectory string

func init() {
	log.SetLevel(log.LevelInfo)

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

	flag.StringVar(&dataDirectory, "data-directory", defaultDataDirectory, "Path to the data directory")
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		log.Infof("Version: %s", Version)
		return
	}

	log.Info("Starting Magi!")

	flag.Parse()

	// Determine cache directory
	var cacheDirectory string
	if cacheDir := os.Getenv("MAGI_CACHE_DIR"); cacheDir != "" {
		cacheDirectory = cacheDir
	} else {
		cacheDirectory = filepath.Join(dataDirectory, "cache")
	}

	// Ensure the directories exist
	if err := os.MkdirAll(cacheDirectory, os.ModePerm); err != nil {
		log.Errorf("Failed to create cache directory: %s", err)
		return
	}

	log.Infof("Using '%s/magi.db,-shm,-wal' as the database location", dataDirectory)
	log.Infof("Using '%s/...' as the image caching location", cacheDirectory)

	// Initialize console log streaming for admin panel
	utils.InitializeConsoleLogger()

	// Initialize database connection
	err := models.Initialize(dataDirectory)
	if err != nil {
		log.Errorf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := models.Close(); err != nil {
			log.Errorf("Failed to close database: %v", err)
		}
	}()

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
	go handlers.Initialize(app, cacheDirectory)

	// Start Indexer in its own goroutine
	libraries, err := models.GetLibraries()
	if err != nil {
		log.Warnf("Failed to get libraries: %v", err)
		return
	}
	go indexer.Initialize(cacheDirectory, libraries)

	// Block main thread to keep goroutines running
	select {}
}
