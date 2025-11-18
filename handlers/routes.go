package handlers

import (
	"strings"

	"github.com/alexander-bruun/magi/executor"
	"github.com/alexander-bruun/magi/indexer"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
)

// savedCacheDirectory stores the cache directory path for image downloads
var savedCacheDirectory string

// Initialize configures all HTTP routes, middleware, and static assets for the application
func Initialize(app *fiber.App, cacheDirectory string) {
	log.Info("Initializing application routes and middleware")

	savedCacheDirectory = cacheDirectory

	// ========================================
	// Set up job status notification callbacks
	// ========================================
	
	executor.NotifyScraperStarted = NotifyScraperStarted
	executor.NotifyScraperFinished = NotifyScraperFinished
	indexer.NotifyIndexerStarted = NotifyIndexerStarted
	indexer.NotifyIndexerProgress = NotifyIndexerProgress
	indexer.NotifyIndexerFinished = NotifyIndexerFinished

	// ========================================
	// Middleware Configuration
	// ========================================
	
	// CORS - Allow all origins
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	// Optional authentication - populates user context when cookies are present
	app.Use(OptionalAuthMiddleware())

	// Health check endpoint
	app.Use(healthcheck.New())

	// Handle CORS preflight requests
	app.Options("/*", func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		return c.SendStatus(fiber.StatusOK)
	})

	// ========================================
	// Static Assets with Cache Headers
	// ========================================
	
	// Cache middleware for downloaded/indexed images
	app.Use("/api/images", func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead {
			// Determine file extension
			p := c.Path() // e.g. /api/images/manga-title.jpg
			ext := ""
			if idx := strings.LastIndex(p, "."); idx != -1 {
				ext = strings.ToLower(p[idx:])
			}

			// Set appropriate cache headers based on file type
			switch ext {
			case ".png", ".jpg", ".jpeg", ".gif", ".webp":
				// Images: cache for 1 year since they're content-addressed
				c.Set("Cache-Control", "public, max-age=31536000, immutable")
			default:
				// Default: no cache
				c.Set("Cache-Control", "public, max-age=0, must-revalidate")
			}
		}
		return c.Next()
	})
	
	app.Static("/api/images", cacheDirectory)
	app.Static("/assets/", "./assets/")

	// ========================================
	// API Endpoints
	// ========================================
	
	api := app.Group("/api")
	
	// Comic book file serving (supports: .cbz, .cbr, .zip, .rar, .jpg, .png)
	api.Get("/comic", ConditionalAuthMiddleware(), ComicHandler)
	
	// Duplicate management (admin only)
	apiAdmin := api.Group("/admin", AuthMiddleware("admin"))
	apiAdmin.Post("/duplicates/:id/dismiss", HandleDismissDuplicate)
	apiAdmin.Get("/duplicates/:id/folder-info", HandleGetDuplicateFolderInfo)
	apiAdmin.Delete("/duplicates/:id/folder", HandleDeleteDuplicateFolder)

	// ========================================
	// Public Routes
	// ========================================
	
	app.Get("/", HandleHome)

	// ========================================
	// Authentication Routes
	// ========================================
	
	auth := app.Group("/auth")
	auth.Get("/login", LoginHandler)
	auth.Post("/login", LoginUserHandler)
	auth.Get("/register", RegisterHandler)
	auth.Post("/register", CreateUserHandler)
	auth.Post("/logout", LogoutHandler)

	// ========================================
	// Manga Routes
	// ========================================
	
	mangas := app.Group("/mangas", ConditionalAuthMiddleware())
	
	// Manga listing and search
	mangas.Get("", HandleMangas)
	mangas.Get("/search", HandleMangaSearch)
	
	// Tag browsing
	mangas.Get("/tags", HandleTags)
	mangas.Get("/tags/fragment", HandleTagsFragment)
	
	// Individual manga
	mangas.Get("/:manga", HandleManga)
	
	// Manga interactions (authenticated)
	mangas.Post("/:manga/vote", AuthMiddleware("reader"), HandleMangaVote)
	mangas.Get("/:manga/vote/fragment", HandleMangaVoteFragment)
	mangas.Post("/:manga/favorite", AuthMiddleware("reader"), HandleMangaFavorite)
	mangas.Get("/:manga/favorite/fragment", HandleMangaFavoriteFragment)
	
	// Manga metadata management (moderator+)
	mangas.Get("/:manga/metadata/form", AuthMiddleware("moderator"), HandleUpdateMetadataManga)
	mangas.Post("/:manga/metadata/manual", AuthMiddleware("moderator"), HandleManualEditMetadata)
	mangas.Post("/:manga/metadata/refresh", AuthMiddleware("moderator"), HandleRefreshMetadata)
	mangas.Post("/:manga/metadata/overwrite", AuthMiddleware("moderator"), HandleEditMetadataManga)
	
	// Poster selector (moderator+)
	mangas.Get("/:manga/poster/chapters", AuthMiddleware("moderator"), HandlePosterChapterSelect)
	mangas.Get("/:manga/poster/selector", AuthMiddleware("moderator"), HandlePosterSelector)
	mangas.Get("/:manga/poster/preview", AuthMiddleware("moderator"), HandlePosterPreview)
	mangas.Post("/:manga/poster/set", AuthMiddleware("moderator"), HandlePosterSet)
	
	// Chapter routes
	chapters := mangas.Group("/:manga/chapters")
	chapters.Get("/:chapter", HandleChapter)
	chapters.Post("/:chapter/read", AuthMiddleware("reader"), HandleMarkRead)
	chapters.Post("/:chapter/unread", AuthMiddleware("reader"), HandleMarkUnread)

	// ========================================
	// User Account Routes (authenticated)
	// ========================================
	
	account := app.Group("/account", AuthMiddleware("reader"))
	account.Get("", HandleAccount)
	account.Get("/favorites", HandleAccountFavorites)
	account.Get("/upvoted", HandleAccountUpvoted)
	account.Get("/downvoted", HandleAccountDownvoted)
	account.Get("/reading", HandleAccountReading)

	// ========================================
	// User Management Routes (moderator+)
	// ========================================
	
	users := app.Group("/admin/users", AuthMiddleware("moderator"))
	users.Get("", HandleUsers)
	users.Post("/:username/ban", HandleUserBan)
	users.Post("/:username/unban", HandleUserUnban)
	users.Post("/:username/promote", HandleUserPromote)
	users.Post("/:username/demote", HandleUserDemote)

	// ========================================
	// Library Management Routes (admin)
	// ========================================
	
	libraries := app.Group("/admin/libraries", AuthMiddleware("admin"))
	libraries.Get("", HandleLibraries)
	libraries.Post("", HandleCreateLibrary)
	libraries.Get("/:slug", HandleEditLibrary)
	libraries.Put("/:slug", HandleUpdateLibrary)
	libraries.Delete("/:slug", HandleDeleteLibrary)
	libraries.Post("/:slug/scan", HandleScanLibrary)
	
	// Library form helpers (HTMX fragments)
	libraries.Get("/helpers/add-folder", HandleAddFolder)
	libraries.Get("/helpers/remove-folder", HandleRemoveFolder)
	libraries.Get("/helpers/cancel-edit", HandleCancelEdit)

	// ========================================
	// Scraper Routes (moderator+)
	// ========================================

	scraper := app.Group("/admin/scraper", AuthMiddleware("moderator"))
	scraper.Get("", HandleScraper)
	scraper.Get("/new", HandleScraperNewForm)
	scraper.Post("", HandleScraperScriptCreate)
	scraper.Get("/:id", HandleScraperScriptDetail)
	scraper.Get("/:id/logs/view", HandleScraperLogs)
	scraper.Put("/:id", HandleScraperScriptUpdate)
	scraper.Delete("/:id", HandleScraperScriptDelete)
	scraper.Post("/:id/run", HandleScraperScriptRun)
	scraper.Post("/:id/toggle", HandleScraperScriptToggle)
	scraper.Post("/:id/cancel", HandleScraperScriptCancel)
	scraper.Get("/:id/logs", HandleScraperLogsWebSocketUpgrade)

	// Scraper Helper Routes
	scraperHelpers := app.Group("/admin/scraper/helpers", AuthMiddleware("moderator"))
	scraperHelpers.Get("/add-variable", HandleScraperVariableAdd)
	scraperHelpers.Get("/remove-variable", HandleScraperVariableRemove)

	// ========================================
	// Duplicate Detection (admin)
	// ========================================
	
	app.Get("/admin/duplicates", AuthMiddleware("admin"), HandleBetter)

	// ========================================
	// Configuration Routes (admin)
	// ========================================
	
	config := app.Group("/admin/config", AuthMiddleware("admin"))
	config.Get("", HandleConfiguration)
	config.Post("", HandleConfigurationUpdate)
	config.Get("/logs", HandleConsoleLogsWebSocketUpgrade)

	// ========================================
	// Job Status WebSocket (public)
	// ========================================
	
	app.Get("/ws/job-status", HandleJobStatusWebSocketUpgrade)

	// ========================================
	// Fallback Route
	// ========================================
	
	app.Get("/*", HandleNotFound)

	// ========================================
	// Start Server
	// ========================================
	
	log.Info("Starting server on port 3000")
	log.Fatal(app.Listen(":3000"))
}
