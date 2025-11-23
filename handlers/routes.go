package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/executor"
	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/utils"
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
	// Start token cleanup goroutine
	// ========================================
	
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			utils.CleanupExpiredTokens()
		}
	}()


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
	app.Use("/api/images", BotDetectionMiddleware(), func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead {
			// Determine file extension
			p := c.Path() // e.g. /api/images/series-title.jpg
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
	
	// Dynamic image serving with WebP conversion support
	app.Get("/api/images/*", BotDetectionMiddleware(), func(c *fiber.Ctx) error {
		return handleImageRequest(c, savedCacheDirectory)
	})
	app.Static("/assets/", "./assets/")

	// ========================================
	// API Endpoints
	// ========================================
	
	api := app.Group("/api")
	
	// Comic book file serving (supports: .cbz, .cbr, .zip, .rar, .jpg, .png)
	api.Get("/comic", BotDetectionMiddleware(), ConditionalAuthMiddleware(), ComicHandler)
	
	// Duplicate management (admin only)
	apiAdmin := api.Group("/admin", AuthMiddleware("admin"))
	apiAdmin.Post("/duplicates/:id/dismiss", HandleDismissDuplicate)
	apiAdmin.Get("/duplicates/:id/folder-info", HandleGetDuplicateFolderInfo)
	apiAdmin.Delete("/duplicates/:id/folder", HandleDeleteDuplicateFolder)

	// ========================================
	// Public Routes
	// ========================================
	
	app.Get("/", HandleHome)
	app.Get("/top-read", HandleTopReadPeriod)
	app.Get("/statistics", HandleStatistics)

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
	// Media Routes
	// ========================================
	
	media := app.Group("/series", ConditionalAuthMiddleware())
	
	// Media listing and search
	media.Get("", HandleMedias)
	media.Get("/search", HandleMediaSearch)
	
	// Tag browsing
	media.Get("/tags", HandleTags)
	media.Get("/tags/fragment", HandleTagsFragment)
	
	// Individual media
	media.Get("/:media", HandleMedia)
	
	// Media interactions (authenticated)
	media.Post("/:media/vote", AuthMiddleware("reader"), HandleMediaVote)
	media.Get("/:media/vote/fragment", HandleMediaVoteFragment)
	media.Post("/:media/favorite", AuthMiddleware("reader"), HandleMediaFavorite)
	media.Get("/:media/favorite/fragment", HandleMediaFavoriteFragment)
	
	// Media metadata management (moderator+)
	media.Get("/:media/metadata/form", AuthMiddleware("moderator"), HandleUpdateMetadataMedia)
	media.Post("/:media/metadata/manual", AuthMiddleware("moderator"), HandleManualEditMetadata)
	media.Post("/:media/metadata/refresh", AuthMiddleware("moderator"), HandleRefreshMetadata)
	media.Post("/:media/metadata/overwrite", AuthMiddleware("moderator"), HandleEditMetadataMedia)
	media.Post("/:media/delete", AuthMiddleware("moderator"), HandleDeleteMedia)
	
	// Poster selector (moderator+)
	media.Get("/:media/poster/chapters", AuthMiddleware("moderator"), HandlePosterChapterSelect)
	media.Get("/:media/poster/selector", AuthMiddleware("moderator"), HandlePosterSelector)
	media.Get("/:media/poster/preview", AuthMiddleware("moderator"), HandlePosterPreview)
	media.Post("/:media/poster/set", AuthMiddleware("moderator"), HandlePosterSet)
	
	// Chapter routes
	chapters := media.Group("/:media")
	chapters.Get("/:chapter/assets/*", HandleMediaChapterAsset)
	chapters.Get("/:chapter/toc", HandleMediaChapterTOC)
	chapters.Get("/:chapter/content", HandleMediaChapterContent)
	chapters.Get("/:chapter", RateLimitingMiddleware(), BotDetectionMiddleware(), HandleChapter)
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
	// Notification Routes (authenticated)
	// ========================================
	
	notifications := app.Group("/api/notifications", AuthMiddleware("reader"))
	notifications.Get("", HandleGetNotifications)
	notifications.Get("/unread-count", HandleGetUnreadCount)
	notifications.Post("/:id/read", HandleMarkNotificationRead)
	notifications.Post("/mark-all-read", HandleMarkAllNotificationsRead)
	notifications.Delete("/:id", HandleDeleteNotification)
	notifications.Delete("/clear-read", HandleClearReadNotifications)

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
	// Banned IPs Management Routes (admin)
	// ========================================
	
	bannedIPs := app.Group("/admin/banned-ips", AuthMiddleware("admin"))
	bannedIPs.Get("", HandleBannedIPs)
	bannedIPs.Post("/:ip/unban", HandleUnbanIP)

	// ========================================
	// Permission Management UI (moderator+)
	// ========================================
	
	app.Get("/admin/permissions", AuthMiddleware("moderator"), HandlePermissionsManagement)

	// ========================================
	// Permission Management Routes (moderator+)
	// ========================================
	
	permissions := app.Group("/api/permissions", AuthMiddleware("moderator"))
	permissions.Get("/list", HandleGetPermissions) // HTMX fragment
	permissions.Get("/:id/form", HandleGetPermissionForm) // Returns form for editing
	permissions.Post("", HandleCreatePermission)
	permissions.Put("/:id", HandleUpdatePermission)
	permissions.Delete("/:id", HandleDeletePermission)
	
	// User permission assignment
	userPerms := app.Group("/api/users", AuthMiddleware("moderator"))
	userPerms.Get("/permissions", HandleGetUserPermissions) // HTMX fragment with ?username=
	userPerms.Post("/permissions/assign", HandleAssignPermissionToUser)
	userPerms.Delete("/:username/permissions/:permissionId", HandleRevokePermissionFromUser)
	
	// Role permission assignment
	rolePerms := app.Group("/api/roles", AuthMiddleware("moderator"))
	rolePerms.Get("/permissions", HandleGetRolePermissions) // HTMX fragment with ?role=
	rolePerms.Post("/permissions/assign", HandleAssignPermissionToRole)
	rolePerms.Delete("/:role/permissions/:permissionId", HandleRevokePermissionFromRole)
	
	// Bulk assignment
	permissions.Get("/:id/bulk-assign", HandleGetBulkAssignForm)
	permissions.Post("/:id/bulk-assign", HandleBulkAssignPermission)

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
	scraperHelpers.Get("/add-package", HandleScraperPackageAdd)
	scraperHelpers.Get("/remove-package", HandleScraperPackageRemove)

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
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	
	log.Debug("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

// handleImageRequest serves images with WebP conversion for better compression
func handleImageRequest(c *fiber.Ctx, cacheDir string) error {
	// Get the requested path (remove /api/images/ prefix)
	imagePath := strings.TrimPrefix(c.Path(), "/api/images/")
	if imagePath == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid image path")
	}

	// Construct full file path
	fullPath := filepath.Join(cacheDir, imagePath)

	// Check if the original file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("Image not found")
	}

	// Always try WebP conversion first for better compression
	if !strings.HasSuffix(strings.ToLower(imagePath), ".webp") {
		// Convert to WebP in memory
		webpData, err := utils.ConvertToWebP(fullPath)
		if err != nil {
			log.Warnf("Failed to convert image to WebP: %v", err)
			// Fall back to original format
		} else {
			// Set WebP content type and serve the converted data
			c.Set("Content-Type", "image/webp")
			c.Set("Cache-Control", "public, max-age=1800") // 30 minutes cache for in-memory conversions
			return c.Send(webpData)
		}
	}

	// Fall back to original format with appropriate content type and cache headers
	switch strings.ToLower(filepath.Ext(imagePath)) {
	case ".jpg", ".jpeg":
		c.Set("Content-Type", "image/jpeg")
	case ".png":
		c.Set("Content-Type", "image/png")
	case ".gif":
		c.Set("Content-Type", "image/gif")
	case ".webp":
		c.Set("Content-Type", "image/webp")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}

	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.SendFile(fullPath)
}
