package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/filestore"
	"github.com/alexander-bruun/magi/scheduler"
	"github.com/alexander-bruun/magi/utils"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
)

// savedDataDirectory stores the data directory path for image downloads
var savedDataDirectory string

// savedBackupDirectory stores the backup directory path
var savedBackupDirectory string

// dataManager manages data operations
var dataManager *filestore.DataManager

// GetDataBackend returns the current data backend
func GetDataBackend() filestore.DataBackend {
	if dataManager == nil {
		return nil
	}
	return dataManager.Backend()
}

// shutdownChan is used to trigger application shutdown
var shutdownChan = make(chan struct{})

// tokenCleanupStop is used to stop the token cleanup goroutine
var tokenCleanupStop = make(chan struct{})

// GetShutdownChan returns the shutdown channel for triggering application shutdown
func GetShutdownChan() <-chan struct{} {
	return shutdownChan
}

// Initialize configures all HTTP routes, middleware, and static assets for the application
func Initialize(app *fiber.App, dataBackend filestore.DataBackend, backupDirectory string, port string) {
	log.Info("Initializing application routes and middleware")

	// Initialize data manager with provided backend
	dataManager = filestore.NewDataManager(dataBackend)

	// Store backup directory for backup operations
	savedBackupDirectory = backupDirectory

	// ========================================
	// Initialize CSS Parser for dynamic CSS injection
	// ========================================
	if err := InitCSSParser("./assets/css"); err != nil {
		log.Warnf("Failed to initialize CSS parser: %v - falling back to static CSS", err)
	}

	// ========================================
	// Initialize JS Cache for dynamic JS inlining
	// ========================================
	if err := InitJSCache("./assets"); err != nil {
		log.Warnf("Failed to initialize JS cache: %v - falling back to static JS", err)
	}

	// ========================================
	// Initialize Image Cache for inlining icons
	// ========================================
	if err := InitImgCache("./assets"); err != nil {
		log.Warnf("Failed to initialize image cache: %v - falling back to static images", err)
	}

	// ========================================
	// Initialize console logger for WebSocket streaming
	// ========================================
	utils.InitializeConsoleLogger()

	// ========================================
	// Set up job status notification callbacks
	// ========================================
	scheduler.NotifyScraperStarted = NotifyScraperStarted
	scheduler.NotifyScraperFinished = NotifyScraperFinished
	scheduler.NotifyIndexerStarted = NotifyIndexerStarted
	scheduler.NotifyIndexerProgress = NotifyIndexerProgress
	scheduler.NotifyIndexerFinished = NotifyIndexerFinished

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
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // Fast compression for better performance
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	app.Use(RequestIDMiddleware())
	app.Use(OptionalAuthMiddleware())
	app.Use(MaintenanceModeMiddleware())
	app.Use(healthcheck.New())

	// CSS Middleware - dynamically injects only required CSS
	app.Use(CSSMiddleware())

	// JS Middleware - dynamically injects only required JS
	app.Use(JSMiddleware())

	// Image Middleware - inlines icon.webp as data URI
	app.Use(ImgMiddleware())

	// ========================================
	// Health and Metrics Endpoints
	// ========================================
	app.Get("/ready", HandleReady)
	app.Get("/health", HandleHealth)
	app.Get("/metrics", AuthMiddleware("admin"), HandleMetrics)
	app.Get("/api/metrics/json", AuthMiddleware("admin"), HandleMetricsJSON)

	app.Options("/*", func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		return c.SendStatus(fiber.StatusOK)
	})

	// ========================================
	// Image Routes with Cache Headers
	// ========================================

	// Dynamic image serving (legacy - returns 404 for cached images)
	// app.Use("/api/images", BotDetectionMiddleware(), imageCacheMiddleware)
	// app.Get("/api/images/*", BotDetectionMiddleware(), func(c *fiber.Ctx) error {
	// 	return handleImageRequest(c)
	// })

	// Poster serving (no bot detection for better UX)
	app.Use("/api/posters", imageCacheMiddleware)
	app.Get("/api/posters/*", func(c *fiber.Ctx) error {
		return handlePosterRequest(c)
	})

	// Avatar serving
	app.Use("/api/avatars", imageCacheMiddleware)
	app.Get("/api/avatars/*", func(c *fiber.Ctx) error {
		return handleAvatarRequest(c)
	})

	// Static assets (CSS and JS are handled by their respective middlewares)
	app.Use("/assets/", func(c *fiber.Ctx) error {
		// Set cache headers for static assets (1 year for JS/CSS, 1 day for images)
		if strings.HasSuffix(c.Path(), ".js") || strings.HasSuffix(c.Path(), ".css") {
			c.Set("Cache-Control", "public, max-age=31536000") // 1 year
		} else if strings.HasPrefix(c.Path(), "/assets/img/") {
			c.Set("Cache-Control", "public, max-age=86400") // 1 day
		}
		return c.Next()
	})
	app.Static("/assets/img/", "./assets/img/")

	// ========================================
	// API Endpoints
	// ========================================
	api := app.Group("/api")

	api.Get("/comic", BotDetectionMiddleware(), ConditionalAuthMiddleware(), ComicHandler)
	api.Get("/image", ImageProtectionMiddleware(), BotDetectionMiddleware(), ConditionalAuthMiddleware(), ImageHandler)

	// Comments
	api.Delete("/comments/:id", AuthMiddleware("reader"), HandleDeleteComment)

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
	app.Get("/external/callback/mal", HandleMALCallback)
	app.Get("/external/callback/anilist", HandleAniListCallback)

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
	// Collections Routes
	// ========================================
	app.Get("/collections", HandleCollections)
	app.Get("/collections/create", AuthMiddleware("reader"), HandleCreateCollectionForm)
	app.Get("/collections/create/modal", AuthMiddleware("reader"), HandleCreateCollectionModal)
	app.Post("/collections/create", AuthMiddleware("reader"), HandleCreateCollection)

	collections := app.Group("/collections/:id", func(c *fiber.Ctx) error {
		idStr := c.Params("id")
		if _, err := strconv.Atoi(idStr); err != nil {
			return c.Status(400).SendString("Invalid collection ID")
		}
		return c.Next()
	})

	collections.Get("", HandleCollection)
	collections.Get("/edit", AuthMiddleware("reader"), HandleEditCollectionForm)
	collections.Get("/edit/modal", AuthMiddleware("reader"), HandleEditCollectionModal)
	collections.Post("/edit", AuthMiddleware("reader"), HandleUpdateCollection)
	collections.Post("/delete", AuthMiddleware("reader"), HandleDeleteCollection)
	collections.Post("/add-media", AuthMiddleware("reader"), HandleAddMediaToCollection)
	collections.Post("/remove-media/:mediaSlug", AuthMiddleware("reader"), HandleRemoveMediaFromCollection)

	// Media collections routes
	app.Get("/series/:media/collections", AuthMiddleware("reader"), HandleGetMediaCollections)
	app.Get("/series/:media/collections/modal", AuthMiddleware("reader"), HandleGetMediaCollectionsModal)
	app.Post("/series/:media/collections/add", AuthMiddleware("reader"), HandleAddMediaToCollectionFromMedia)
	app.Post("/series/:media/collections/remove", AuthMiddleware("reader"), HandleRemoveMediaFromCollectionFromMedia)

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

	// Comments and reviews
	media.Get("/:media<[A-Za-z0-9_-]+>/comments", HandleGetComments)
	media.Post("/:media<[A-Za-z0-9_-]+>/comments", AuthMiddleware("reader"), HandleCreateComment)
	media.Get("/:media<[A-Za-z0-9_-]+>/reviews", HandleGetReviews)
	media.Post("/:media<[A-Za-z0-9_-]+>/reviews", AuthMiddleware("reader"), HandleCreateReview)
	media.Get("/:media<[A-Za-z0-9_-]+>/reviews/user", AuthMiddleware("reader"), HandleGetUserReview)
	media.Delete("/:media<[A-Za-z0-9_-]+>/reviews/:reviewId?", AuthMiddleware("reader"), HandleDeleteReview)

	// Metadata routes
	media.Get("/:media/metadata/form", AuthMiddleware("moderator"), HandleUpdateMetadataMedia)
	media.Post("/:media/metadata/manual", AuthMiddleware("moderator"), HandleManualEditMetadata)
	media.Post("/:media/metadata/refresh", AuthMiddleware("moderator"), HandleRefreshMetadata)
	media.Post("/:media/metadata/reindex", AuthMiddleware("moderator"), HandleReindexChapters)
	media.Post("/:media/metadata/overwrite", AuthMiddleware("moderator"), HandleEditMetadataMedia)
	media.Post("/:media<[A-Za-z0-9_-]+>/delete", AuthMiddleware("moderator"), HandleDeleteMedia)

	// Poster selector
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/chapters", ConditionalAuthMiddleware(), HandlePosterChapterSelect)
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/selector", ConditionalAuthMiddleware(), HandlePosterSelector)
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/preview", ConditionalAuthMiddleware(), HandlePosterPreview)
	media.Post("/:media<[A-Za-z0-9_-]+>/poster/set", ConditionalAuthMiddleware(), HandlePosterSet)

	// Highlight management
	media.Post("/:media<[A-Za-z0-9_-]+>/highlights/add", AuthMiddleware("moderator"), HandleAddHighlight)
	media.Post("/:media<[A-Za-z0-9_-]+>/highlights/remove", AuthMiddleware("moderator"), HandleRemoveHighlight)

	// Chapter routes (slug restricted)
	chapters := media.Group("/:media<[A-Za-z0-9_-]+>")

	chapters.Get("/:chapter/assets/*", HandleMediaChapterAsset)
	chapters.Get("/:chapter/toc", HandleMediaChapterTOC)
	chapters.Get("/:chapter/content", HandleMediaChapterContent)
	chapters.Get("/:chapter", RateLimitingMiddleware(), BotDetectionMiddleware(), HandleChapter)
	chapters.Post("/:chapter/read", AuthMiddleware("reader"), HandleMarkRead)
	chapters.Post("/:chapter/unread", AuthMiddleware("reader"), HandleMarkUnread)
	chapters.Post("/:chapter/unmark-premium", AuthMiddleware("moderator"), HandleUnmarkChapterPremium)

	// Chapter comments
	chapters.Get("/:chapter/comments", HandleGetComments)
	chapters.Post("/:chapter/comments", AuthMiddleware("reader"), HandleCreateComment)

	// ========================================
	// Account Routes
	// ========================================
	account := app.Group("/account", AuthMiddleware("reader"))
	account.Get("", HandleAccount)
	account.Get("/favorites", HandleAccountFavorites)
	account.Get("/upvoted", HandleAccountUpvoted)
	account.Get("/downvoted", HandleAccountDownvoted)
	account.Get("/reading", HandleAccountReading)
	account.Get("/external", HandleExternalAccounts)
	account.Post("/avatar", HandleUploadAvatar)
	account.Post("/external/mal/connect", HandleConnectMAL)
	account.Get("/external/mal/authorize", HandleAuthorizeMAL)
	account.Post("/external/mal/disconnect", HandleDisconnectMAL)
	account.Post("/external/anilist/connect", HandleConnectAniList)
	account.Get("/external/anilist/authorize", HandleAuthorizeAniList)
	account.Post("/external/anilist/disconnect", HandleDisconnectAniList)

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
	users.Get("/table", HandleUsersTable)
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
	app.Get("/admin/monitoring", AuthMiddleware("moderator"), HandleMonitoring)

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
	libraries.Get("/:slug/logs", HandleIndexerLogsWebSocketUpgrade)
	libraries.Get("/helpers/add-folder", HandleAddFolder)
	libraries.Get("/helpers/remove-folder", HandleRemoveFolder)
	libraries.Get("/helpers/cancel-edit", HandleCancelEdit)
	libraries.Get("/helpers/browse", HandleBrowseDirectory)

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
	scraper.Delete("/:id/logs/:logId", HandleScraperLogDelete)
	scraper.Post("/:id/run", HandleScraperScriptRun)
	scraper.Post("/:id/toggle", HandleScraperScriptToggle)
	scraper.Post("/:id/cancel", HandleScraperScriptCancel)
	scraper.Get("/:id/logs", HandleScraperLogsWebSocketUpgrade)

	scraperHelpers := app.Group("/admin/scraper/helpers", AuthMiddleware("moderator"))
	scraperHelpers.Get("/add-variable", HandleScraperVariableAdd)
	scraperHelpers.Get("/remove-variable", HandleScraperVariableRemove)
	scraperHelpers.Get("/add-package", HandleScraperPackageAdd)
	scraperHelpers.Get("/remove-package", HandleScraperPackageRemove)
	scraperHelpers.Get("/update-language", HandleScraperUpdateLanguage)

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
