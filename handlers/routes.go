package handlers

import (
	"strings"
	"time"

	"github.com/alexander-bruun/magi/executor"
	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/utils"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	
)

// savedCacheDirectory stores the cache directory path for image downloads
var savedCacheDirectory string

// savedBackupDirectory stores the backup directory path
var savedBackupDirectory string

// shutdownChan is used to trigger application shutdown
var shutdownChan = make(chan struct{})

// tokenCleanupStop is used to stop the token cleanup goroutine
var tokenCleanupStop = make(chan struct{})

// GetShutdownChan returns the shutdown channel for triggering application shutdown
func GetShutdownChan() <-chan struct{} {
	return shutdownChan
}

// Initialize configures all HTTP routes, middleware, and static assets for the application
func Initialize(app *fiber.App, cacheDirectory string, backupDirectory string, port string) {
	log.Info("Initializing application routes and middleware")

	savedCacheDirectory = cacheDirectory
	savedBackupDirectory = backupDirectory

	// ========================================
	// Initialize console logger for WebSocket streaming
	// ========================================
	utils.InitializeConsoleLogger()

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
		for {
			select {
			case <-ticker.C:
				utils.CleanupExpiredTokens()
			case <-tokenCleanupStop:
				return
			}
		}
	}()

	// ========================================
	// Middleware Configuration
	// ========================================
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	app.Use(OptionalAuthMiddleware())
	app.Use(healthcheck.New())

	// ========================================
	// Health and Metrics Endpoints
	// ========================================
	app.Get("/ready", HandleReady)
	app.Get("/health", HandleHealth)
	app.Get("/metrics", HandleMetrics)

	app.Options("/*", func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		return c.SendStatus(fiber.StatusOK)
	})

	// ========================================
	// Static Assets with Cache Headers
	// ========================================
	app.Use("/api/images", BotDetectionMiddleware(), func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead {
			p := c.Path()
			ext := ""
			if idx := strings.LastIndex(p, "."); idx != -1 {
				ext = strings.ToLower(p[idx:])
			}

			switch ext {
			case ".png", ".jpg", ".jpeg", ".gif", ".webp":
				c.Set("Cache-Control", "public, max-age=31536000, immutable")
			default:
				c.Set("Cache-Control", "public, max-age=0, must-revalidate")
			}
		}
		return c.Next()
	})

	// Dynamic image serving
	app.Get("/api/images/*", BotDetectionMiddleware(), func(c *fiber.Ctx) error {
		return handleImageRequest(c)
	})

	// Poster serving (no bot detection for better UX)
	app.Use("/api/posters", func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead {
			p := c.Path()
			ext := ""
			if idx := strings.LastIndex(p, "."); idx != -1 {
				ext = strings.ToLower(p[idx:])
			}

			switch ext {
			case ".png", ".jpg", ".jpeg", ".gif", ".webp":
				c.Set("Cache-Control", "public, max-age=31536000, immutable")
			default:
				c.Set("Cache-Control", "public, max-age=0, must-revalidate")
			}
		}
		return c.Next()
	})
	app.Get("/api/posters/*", func(c *fiber.Ctx) error {
		return handleImageRequest(c)
	})

	// Static assets (must be early)
	app.Static("/assets/", "./assets/")

	// ========================================
	// API Endpoints
	// ========================================
	api := app.Group("/api")

	api.Get("/comic", BotDetectionMiddleware(), ConditionalAuthMiddleware(), ComicHandler)
	api.Get("/image", ImageProtectionMiddleware(), BotDetectionMiddleware(), ConditionalAuthMiddleware(), ImageHandler)

	apiAdmin := api.Group("/admin", AuthMiddleware("admin"))
	apiAdmin.Post("/duplicates/:id/dismiss", HandleDismissDuplicate)
	apiAdmin.Get("/duplicates/:id/folder-info", HandleGetDuplicateFolderInfo)
	apiAdmin.Delete("/duplicates/:id/folder", HandleDeleteDuplicateFolder)

	// ========================================
	// Captcha Routes
	// ========================================
	app.Get("/captcha/:id.png", HandleCaptchaImage)
	app.Get("/captcha/new", HandleCaptchaNew)
	app.Post("/captcha/verify", HandleCaptchaVerify)
	app.Get("/captcha", HandleCaptchaPage)

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
	// Media Routes (FIXED)
	// ========================================

	// Guard middleware to reject bad slugs before they reach handlers
	media := app.Group("/series",
		ConditionalAuthMiddleware(),
		func(c *fiber.Ctx) error {
			// Extract first segment after /series/
			rest := strings.TrimPrefix(c.Path(), "/series/")
			if rest == "" || rest == "/" {
				return c.Next()
			}
			slug := strings.Split(rest, "/")[0]

			// Reject invalid slugs
			if strings.ContainsAny(slug, "./") {
				return c.SendStatus(404)
			}
			return c.Next()
		},
	)

	// Media listing and search
	media.Get("", HandleMedias)
	media.Get("/search", HandleMediaSearch)

	// Tag browsing
	media.Get("/tags", HandleTags)
	media.Get("/tags/fragment", HandleTagsFragment)

	// Individual media (restricted slug)
	media.Get("/:media<[A-Za-z0-9_-]+>", HandleMedia)

	// Media interactions
	media.Post("/:media<[A-Za-z0-9_-]+>/vote", AuthMiddleware("reader"), HandleMediaVote)
	media.Get("/:media<[A-Za-z0-9_-]+>/vote/fragment", HandleMediaVoteFragment)
	media.Post("/:media<[A-Za-z0-9_-]+>/favorite", AuthMiddleware("reader"), HandleMediaFavorite)
	media.Get("/:media<[A-Za-z0-9_-]+>/favorite/fragment", HandleMediaFavoriteFragment)

	// Metadata routes
	media.Get("/:media/metadata/form", AuthMiddleware("moderator"), HandleUpdateMetadataMedia)
	media.Post("/:media/metadata/manual", AuthMiddleware("moderator"), HandleManualEditMetadata)
	media.Post("/:media/metadata/refresh", AuthMiddleware("moderator"), HandleRefreshMetadata)
	media.Post("/:media/metadata/overwrite", AuthMiddleware("moderator"), HandleEditMetadataMedia)
	media.Post("/:media<[A-Za-z0-9_-]+>/delete", AuthMiddleware("moderator"), HandleDeleteMedia)

	// Poster selector
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/chapters", AuthMiddleware("moderator"), HandlePosterChapterSelect)
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/selector", AuthMiddleware("moderator"), HandlePosterSelector)
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/preview", AuthMiddleware("moderator"), HandlePosterPreview)
	media.Post("/:media<[A-Za-z0-9_-]+>/poster/set", AuthMiddleware("moderator"), HandlePosterSet)

	// Chapter routes (slug restricted)
	chapters := media.Group("/:media<[A-Za-z0-9_-]+>")

	chapters.Get("/:chapter/assets/*", HandleMediaChapterAsset)
	chapters.Get("/:chapter/toc", HandleMediaChapterTOC)
	chapters.Get("/:chapter/content", HandleMediaChapterContent)
	chapters.Get("/:chapter", RateLimitingMiddleware(), BotDetectionMiddleware(), HandleChapter)
	chapters.Post("/:chapter/read", AuthMiddleware("reader"), HandleMarkRead)
	chapters.Post("/:chapter/unread", AuthMiddleware("reader"), HandleMarkUnread)
	chapters.Post("/:chapter/unmark-premium", AuthMiddleware("moderator"), HandleUnmarkChapterPremium)

	// ========================================
	// Account Routes
	// ========================================
	account := app.Group("/account", AuthMiddleware("reader"))
	account.Get("", HandleAccount)
	account.Get("/favorites", HandleAccountFavorites)
	account.Get("/upvoted", HandleAccountUpvoted)
	account.Get("/downvoted", HandleAccountDownvoted)
	account.Get("/reading", HandleAccountReading)

	// ========================================
	// Notification Routes
	// ========================================
	notifications := app.Group("/api/notifications", AuthMiddleware("reader"))
	notifications.Get("", HandleGetNotifications)
	notifications.Get("/unread-count", HandleGetUnreadCount)
	notifications.Post("/:id/read", HandleMarkNotificationRead)
	notifications.Post("/mark-all-read", HandleMarkAllNotificationsRead)
	notifications.Delete("/:id", HandleDeleteNotification)
	notifications.Delete("/clear-read", HandleClearReadNotifications)

	// ========================================
	// User Management Routes
	// ========================================
	users := app.Group("/admin/users", AuthMiddleware("moderator"))
	users.Get("", HandleUsers)
	users.Post("/:username/ban", HandleUserBan)
	users.Post("/:username/unban", HandleUserUnban)
	users.Post("/:username/promote", HandleUserPromote)
	users.Post("/:username/demote", HandleUserDemote)

	// ========================================
	// Banned IPs Management
	// ========================================
	bannedIPs := app.Group("/admin/banned-ips", AuthMiddleware("admin"))
	bannedIPs.Get("", HandleBannedIPs)
	bannedIPs.Post("/:ip/unban", HandleUnbanIP)

	// ========================================
	// Permissions UI
	// ========================================
	app.Get("/admin/permissions", AuthMiddleware("moderator"), HandlePermissionsManagement)

	// ========================================
	// Permission Management API
	// ========================================
	permissions := app.Group("/api/permissions", AuthMiddleware("moderator"))
	permissions.Get("/list", HandleGetPermissions)
	permissions.Get("/:id/form", HandleGetPermissionForm)
	permissions.Post("", HandleCreatePermission)
	permissions.Put("/:id", HandleUpdatePermission)
	permissions.Delete("/:id", HandleDeletePermission)

	userPerms := app.Group("/api/users", AuthMiddleware("moderator"))
	userPerms.Get("/permissions", HandleGetUserPermissions)
	userPerms.Post("/permissions/assign", HandleAssignPermissionToUser)
	userPerms.Delete("/:username/permissions/:permissionId", HandleRevokePermissionFromUser)

	rolePerms := app.Group("/api/roles", AuthMiddleware("moderator"))
	rolePerms.Get("/permissions", HandleGetRolePermissions)
	rolePerms.Post("/permissions/assign", HandleAssignPermissionToRole)
	rolePerms.Delete("/:role/permissions/:permissionId", HandleRevokePermissionFromRole)

	permissions.Get("/:id/bulk-assign", HandleGetBulkAssignForm)
	permissions.Post("/:id/bulk-assign", HandleBulkAssignPermission)

	// ========================================
	// Library Management
	// ========================================
	libraries := app.Group("/admin/libraries", AuthMiddleware("admin"))
	libraries.Get("", HandleLibraries)
	libraries.Post("", HandleCreateLibrary)
	libraries.Get("/:slug", HandleEditLibrary)
	libraries.Put("/:slug", HandleUpdateLibrary)
	libraries.Delete("/:slug", HandleDeleteLibrary)
	libraries.Post("/:slug/scan", HandleScanLibrary)
	libraries.Get("/helpers/add-folder", HandleAddFolder)
	libraries.Get("/helpers/remove-folder", HandleRemoveFolder)
	libraries.Get("/helpers/cancel-edit", HandleCancelEdit)

	// ========================================
	// Scraper Routes
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

	scraperHelpers := app.Group("/admin/scraper/helpers", AuthMiddleware("moderator"))
	scraperHelpers.Get("/add-variable", HandleScraperVariableAdd)
	scraperHelpers.Get("/remove-variable", HandleScraperVariableRemove)
	scraperHelpers.Get("/add-package", HandleScraperPackageAdd)
	scraperHelpers.Get("/remove-package", HandleScraperPackageRemove)

	// ========================================
	// Duplicate Detection
	// ========================================
	app.Get("/admin/duplicates", AuthMiddleware("admin"), HandleBetter)

	// ========================================
	// Configuration
	// ========================================
	config := app.Group("/admin/config", AuthMiddleware("admin"))
	config.Get("", HandleConfiguration)
	config.Post("", HandleConfigurationUpdate)
	config.Get("/logs", HandleConsoleLogsWebSocketUpgrade)

	// ========================================
	// Backups
	// ========================================
	backups := app.Group("/admin/backups", AuthMiddleware("admin"))
	backups.Get("", HandleBackups)
	backups.Post("/create", HandleCreateBackup)
	backups.Post("/restore/:filename", HandleRestoreBackup)

	// ========================================
	// Job Status WebSocket
	// ========================================
	app.Get("/ws/job-status", HandleJobStatusWebSocketUpgrade)

	// ========================================
	// Fallback Route
	// ========================================
	app.Get("/*", HandleNotFound)

	// ========================================
	// Start Server
	// ========================================
	if port == "" {
		port = "3000"
	}

	log.Debug("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

// StopTokenCleanup stops the token cleanup goroutine
func StopTokenCleanup() {
	close(tokenCleanupStop)
}

