package main

import (
	// _ "net/http/pprof" // Import for side-effect of registering pprof handlers

	"embed"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"os"
	"path/filepath"
	"runtime"

	"github.com/alexander-bruun/magi/handlers"
	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/models"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
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

	flag.StringVar(&dataDirectory, "data-directory", defaultDataDirectory, "Path to the data directory")
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		log.Infof("Version: %s", Version)
		return
	}

	log.Info("Starting Magi!")

	flag.Parse()

	// Cache directory under the data directory
	joinedCacheDataDirectory := filepath.Join(dataDirectory, "cache")

	// Ensure the directories exist
	if err := os.MkdirAll(joinedCacheDataDirectory, os.ModePerm); err != nil {
		log.Errorf("Failed to create directories: %s", err)
		return
	}

	log.Debugf("Using '%s/magi.db' as the database location", dataDirectory)
	log.Debugf("Using '%s' as the image caching location", joinedCacheDataDirectory)

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

	// Retrieve or generate JWT key
	_, err = models.GetKey()
	if err != nil {
		log.Info("Error retrieving JWT key:", err)
		key, err := models.GenerateRandomKey(32)
		if err != nil {
			log.Fatal("Failed to generate JWT key:", err)
		}
		if err := models.StoreKey(key); err != nil {
			log.Fatal("Failed to store JWT key:", err)
		}
		log.Info("New JWT key generated and stored")
	} else {
		log.Info("JWT key retrieved from database store")
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

	// Serve embedded assets but add cache headers based on file extension.
	assetsFSHandler := filesystem.New(filesystem.Config{
		Root:       http.FS(assetsfs),
		PathPrefix: "assets",
		Browse:     true,
	})

	// Wrap the fileserver to set Cache-Control headers for static assets.
	app.Use("/assets", func(c *fiber.Ctx) error {
		// Only set caching for GET/HEAD requests
		if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead {
			// Determine extension
			p := c.Path() // e.g. /assets/js/site.js
			ext := ""
			if idx := strings.LastIndex(p, "."); idx != -1 {
				ext = strings.ToLower(p[idx:])
			}

			// Default: no-cache for unknowns
			cacheHeader := "public, max-age=0, must-revalidate"

			switch ext {
			case ".js", ".css", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".map":
				// Long cache for static, fingerprinted assets
				cacheHeader = "public, max-age=31536000, immutable"
			case ".json" :
				cacheHeader = "public, max-age=3600"
			case ".html", "":
				cacheHeader = "public, max-age=0, must-revalidate"
			}

			c.Set("Cache-Control", cacheHeader)
		}

		return assetsFSHandler(c)
	})

	// Start API in its own goroutine
	go handlers.Initialize(app, joinedCacheDataDirectory)

	// Start Indexer in its own goroutine
	libraries, err := models.GetLibraries()
	if err != nil {
		log.Warnf("Failed to get libraries: %v", err)
		return
	}
	go indexer.Initialize(joinedCacheDataDirectory, libraries)

	// Block main thread to keep goroutines running
	select {}
}
