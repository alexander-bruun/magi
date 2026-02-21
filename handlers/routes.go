package handlers

import (
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/alexander-bruun/magi/scheduler"
	"github.com/alexander-bruun/magi/utils/store"
	"github.com/alexander-bruun/magi/utils/text"
	fiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
)

// savedBackupDirectory stores the backup directory path
var savedBackupDirectory string

// fileStore provides local file system operations
var fileStore *store.FileStore

// GetFileStore returns the current file store
func GetFileStore() *store.FileStore {
	return fileStore
}

// shutdownChan is used to trigger application shutdown
var shutdownChan = make(chan struct{})

// GetShutdownChan returns the shutdown channel for triggering application shutdown
func GetShutdownChan() <-chan struct{} {
	return shutdownChan
}

// Initialize configures all HTTP routes, middleware, and static assets for the application
func Initialize(app *fiber.App, fs *store.FileStore, backupDirectory string, port string) {
	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
		log.Info("Initializing application routes and middleware")
	}

	fileStore = fs

	// Store backup directory for backup operations
	savedBackupDirectory = backupDirectory

	var (
		api           fiber.Router
		auth          fiber.Router
		collections   fiber.Router
		media         fiber.Router
		account       fiber.Router
		notifications fiber.Router
		users         fiber.Router
		bannedIPs     fiber.Router
		permissions   fiber.Router
		libraries     fiber.Router
		scraper       fiber.Router
	)

	// ========================================
	// Initialize CSS Parser for dynamic CSS injection
	// ========================================
	if err := InitCSSParser("assets/css"); err != nil {
		log.Warnf("Failed to initialize CSS parser: %v - falling back to static CSS", err)
	}

	// ========================================
	// Initialize JS Cache for dynamic JS inlining
	// ========================================
	if err := InitJSCache("assets"); err != nil {
		log.Warnf("Failed to initialize JS cache: %v - falling back to static JS", err)
	}

	// ========================================
	// Initialize Minifier for CSS, HTML, and JS minification
	// ========================================
	InitMinifier()

	// ========================================
	// Initialize Image Cache for inlining icons
	// ========================================
	if err := InitImgCache("assets"); err != nil {
		log.Warnf("Failed to initialize image cache: %v - falling back to static images", err)
	}

	// ========================================
	// Initialize console logger for WebSocket streaming
	// ========================================
	text.InitializeConsoleLogger()

	// ========================================
	// Set up job status notification callbacks
	// ========================================
	scheduler.NotifyScraperStarted = NotifyScraperStarted
	scheduler.NotifyScraperFinished = NotifyScraperFinished
	scheduler.NotifyIndexerStarted = NotifyIndexerStarted
	scheduler.NotifyIndexerProgress = NotifyIndexerProgress
	scheduler.NotifyIndexerFinished = NotifyIndexerFinished

	// ========================================
	// Middleware Configuration
	// ========================================
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // Fast compression for better performance
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}))

	app.Use(RequestIDMiddleware())
	app.Use(OptionalAuthMiddleware())
	app.Use(MaintenanceModeMiddleware())
	app.Get(healthcheck.LivenessEndpoint, healthcheck.New())

	// Browser Challenge Middleware - serves challenge page for unverified HTML requests
	// This must come before other middlewares to intercept requests early
	app.Use(BrowserChallengePageMiddleware())

	// Referer Validation Middleware - validates request origins
	app.Use(RefererValidationMiddleware())

	// Tarpit Middleware - slows responses for suspected bots
	app.Use(TarpitMiddleware())

	// Request Timing Analysis Middleware - detects bot-like timing patterns
	app.Use(RequestTimingMiddleware())

	// TLS Fingerprint Middleware - analyzes TLS characteristics
	app.Use(TLSFingerprintMiddleware())

	// Behavioral Analysis Middleware - tracks user behavior patterns
	app.Use(BehavioralAnalysisMiddleware())

	// Header Analysis Middleware - checks HTTP headers for bot patterns
	app.Use(HeaderAnalysisMiddleware())

	// Honeypot Middleware - catches scrapers probing for exploits
	app.Use(HoneypotMiddleware())

	// CSS and JS Middleware - optimizes CSS and inlines JS, then minifies all content
	app.Use(MinifyMiddleware())

	// Image Middleware - inlines icon.webp as data URI
	app.Use(ImgMiddleware())

	// ========================================
	// Health and Metrics Endpoints
	// ========================================
	app.Get("/ready", HandleReady)
	app.Get("/health", HandleHealth)
	app.Get("/metrics", AuthMiddleware("admin"), HandleMetrics)
	app.Get("/api/metrics/json", AuthMiddleware("admin"), HandleMetricsJSON)

	app.Options("/*", func(c fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		return c.SendStatus(fiber.StatusOK)
	})

	// ========================================
	// Image Routes with Cache Headers
	// ========================================

	// Poster serving (no bot detection for better UX)
	app.Use("/api/posters", imageCacheMiddleware)
	app.Get("/api/posters/*", func(c fiber.Ctx) error {
		return handlePosterRequest(c)
	})

	// Avatar serving
	app.Use("/api/avatars", imageCacheMiddleware)
	app.Get("/api/avatars/*", func(c fiber.Ctx) error {
		return handleAvatarRequest(c)
	})

	// Static assets (CSS and JS are handled by their respective middlewares)
	app.Use("/assets/", func(c fiber.Ctx) error {
		// Set cache headers for static assets (1 year for JS/CSS, 1 day for images)
		if strings.HasSuffix(c.Path(), ".js") || strings.HasSuffix(c.Path(), ".css") {
			c.Set("Cache-Control", "public, max-age=31536000") // 1 year
		} else if strings.HasPrefix(c.Path(), "/assets/img/") {
			c.Set("Cache-Control", "public, max-age=86400") // 1 day
		}
		return c.Next()
	})
	app.Get("/assets/img/*", static.New("./assets/img/"))

	// Robots.txt
	app.Get("/robots.txt", func(c fiber.Ctx) error {
		return c.SendFile("./assets/robots.txt")
	})

	// PWA manifest and service worker
	app.Get("/assets/manifest.json", func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/manifest+json")
		return c.SendFile("./assets/manifest.json")
	})

	app.Get("/assets/js/sw.js", func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/javascript")
		return c.SendFile("./assets/js/sw.js")
	})

	// ========================================
	// API Endpoints
	// ========================================
	api = app.Group("/api")

	api.Get("/config/stripe", HandleGetStripeConfig)

	// Browser Challenge API (invisible JS/cookie challenge)
	api.Get("/browser-challenge/init", HandleBrowserChallengeInit)
	api.Post("/browser-challenge/verify", HandleBrowserChallengeVerify)

	// Behavioral Analysis API
	api.Post("/behavior-signal", HandleBehaviorSignal)

	// Comments
	api.Delete("/comments/:id", AuthMiddleware("reader"), HandleDeleteComment)

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
	app.Get("/top-read-full", HandleTopReadFull)
	app.Get("/top-read-card", HandleTopReadCard)
	app.Get("/external/callback/mal", HandleMALCallback)
	app.Get("/external/callback/anilist", HandleAniListCallback)

	// ========================================
	// Issue Reporting Routes
	// ========================================
	app.Get("/report-issue", HandleReportIssue)
	app.Post("/report-issue", AuthMiddleware("reader"), HandleCreateIssue)
	app.Get("/report-issue/success", HandleReportIssueSuccess)

	// ========================================
	// Authentication Routes
	// ========================================
	auth = app.Group("/auth")
	auth.Get("/login", loginHandler)
	auth.Post("/login", loginUserHandler)
	auth.Get("/register", registerHandler)
	auth.Post("/register", createUserHandler)
	auth.Post("/logout", logoutHandler)

	// ========================================
	// Collections Routes
	// ========================================
	app.Get("/collections", HandleCollections)
	app.Get("/collections/create", AuthMiddleware("reader"), HandleCreateCollectionForm)
	app.Get("/collections/create/modal", AuthMiddleware("reader"), HandleCreateCollectionModal)
	app.Post("/collections/create", AuthMiddleware("reader"), HandleCreateCollection)

	collections = app.Group("/collections/:id", func(c fiber.Ctx) error {
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
	// Chapter Image Serving (encrypted slug URLs)
	// Must be registered before the /series group to ensure proper route matching.
	// Min-length constraint on :slug avoids conflicts with short path segments like "comments", "chapters", etc.
	// ========================================
	app.Get("/series/:media<[A-Za-z0-9_-]+>/:chapter<[A-Za-z0-9_-]+>/:slug<[A-Za-z0-9_-]{30,}>", BotDetectionMiddleware(), BrowserChallengeMiddleware(), ConditionalAuthMiddleware(), ChapterImageHandler)

	// ========================================
	// Chapter Routes (top-level)
	// ========================================
	app.Get("/chapter/:hash<[A-Za-z0-9_-]+>/assets/*", HandleMediaChapterAsset)
	app.Get("/chapter/:hash<[A-Za-z0-9_-]+>/toc", HandleMediaChapterTOC)
	app.Get("/chapter/:hash<[A-Za-z0-9_-]+>/content", HandleMediaChapterContent)
	app.Get("/chapter/:hash<[A-Za-z0-9_-]+>", RateLimitingMiddleware(), BotDetectionMiddleware(), HandleChapter)
	app.Post("/chapter/:hash<[A-Za-z0-9_-]+>/read", AuthMiddleware("reader"), HandleMarkRead)
	app.Post("/chapter/:hash<[A-Za-z0-9_-]+>/unread", AuthMiddleware("reader"), HandleMarkUnread)
	app.Post("/chapter/:hash<[A-Za-z0-9_-]+>/unmark-premium", AuthMiddleware("moderator"), HandleUnmarkChapterPremium)

	// Chapter comments
	app.Get("/chapter/:hash<[A-Za-z0-9_-]+>/comments", HandleGetComments)
	app.Post("/chapter/:hash<[A-Za-z0-9_-]+>/comments", AuthMiddleware("reader"), HandleCreateComment)

	// ========================================
	// Media Routes (FIXED)
	// ========================================

	// Guard middleware to reject bad slugs before they reach handlers
	media = app.Group("/series",
		ConditionalAuthMiddleware(),
		func(c fiber.Ctx) error {
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

	// Individual media (restricted slug) - registered after chapter routes to avoid conflicts
	media.Get("/:media<[A-Za-z0-9_-]+>", HandleMedia)
	media.Post("/:media<[A-Za-z0-9_-]+>", HandleMedia)

	// Media interactions
	media.Post("/:media<[A-Za-z0-9_-]+>/vote", HandleMediaVote)
	media.Get("/:media<[A-Za-z0-9_-]+>/vote/fragment", HandleMediaVoteFragment)
	media.Post("/:media<[A-Za-z0-9_-]+>/favorite", HandleMediaFavorite)
	media.Get("/:media<[A-Za-z0-9_-]+>/favorite/fragment", HandleMediaFavoriteFragment)
	media.Post("/:media<[A-Za-z0-9_-]+>/highlights/add", AuthMiddleware("moderator"), HandleAddHighlight)
	media.Post("/:media<[A-Za-z0-9_-]+>/highlights/remove", AuthMiddleware("moderator"), HandleRemoveHighlight)

	// Poster management
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/chapters", AuthMiddleware("moderator"), HandlePosterChapterSelect)
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/selector", AuthMiddleware("moderator"), HandlePosterSelector)
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/preview", AuthMiddleware("moderator"), HandlePosterPreview)
	media.Get("/:media<[A-Za-z0-9_-]+>/poster/metadata", AuthMiddleware("moderator"), HandlePosterMetadataSelect)
	media.Post("/:media<[A-Za-z0-9_-]+>/poster/set", AuthMiddleware("moderator"), HandlePosterSet)

	// Media metadata management
	media.Get("/:media<[A-Za-z0-9_-]+>/metadata/search", AuthMiddleware("admin"), HandleUpdateMetadataMedia)
	media.Post("/:media<[A-Za-z0-9_-]+>/metadata/edit", AuthMiddleware("admin"), HandleManualEditMetadata)
	media.Post("/:media<[A-Za-z0-9_-]+>/metadata/reindex", AuthMiddleware("admin"), HandleReindexChapters)
	media.Post("/:media<[A-Za-z0-9_-]+>/metadata/refresh", AuthMiddleware("admin"), HandleRefreshMetadata)
	media.Post("/:media<[A-Za-z0-9_-]+>/delete", AuthMiddleware("admin"), HandleDeleteMedia)

	// Review management - DEPRECATED
	// media.Get("/:media<[A-Za-z0-9_-]+>/reviews", HandleGetReviews)
	// media.Post("/:media<[A-Za-z0-9_-]+>/reviews", AuthMiddleware("reader"), HandleCreateReview)
	// media.Get("/:media<[A-Za-z0-9_-]+>/reviews/user", AuthMiddleware("reader"), HandleGetUserReview)
	// media.Delete("/:media<[A-Za-z0-9_-]+>/reviews/:id", AuthMiddleware("reader"), HandleDeleteReview)

	// Chapter comments
	media.Get("/:media<[A-Za-z0-9_-]+>/chapter-:chapter/comments", HandleGetCommentsForSeries)
	media.Post("/:media<[A-Za-z0-9_-]+>/chapter-:chapter/comments", AuthMiddleware("reader"), HandleCreateCommentForSeries)

	// Chapter page (slug-based URL) - must be registered AFTER all specific media sub-routes
	media.Get("/:media<[A-Za-z0-9_-]+>/:chapter<[A-Za-z0-9_-]+>", RateLimitingMiddleware(), BotDetectionMiddleware(), HandleChapterBySlug)

	// ========================================
	// Account Routes
	// ========================================
	account = app.Group("/account", AuthMiddleware("reader"))
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
	// Premium Routes
	// ========================================
	app.Get("/premium", AuthMiddleware("reader"), HandlePremiumPage)
	app.Post("/premium/create-checkout-session", AuthMiddleware("reader"), HandleCreateCheckoutSession)
	app.Get("/premium/success", AuthMiddleware("reader"), HandlePremiumSuccess)
	app.Post("/premium/cancel", AuthMiddleware("premium"), HandlePremiumCancel)
	app.Post("/api/stripe/webhook", HandleStripeWebhook)

	// ========================================
	// Notification Routes
	// ========================================
	notifications = app.Group("/api/notifications", AuthMiddleware("reader"))
	notifications.Get("", HandleGetNotifications)
	notifications.Get("/unread-count", HandleGetUnreadCount)
	notifications.Post("/:id/read", HandleMarkNotificationRead)
	notifications.Post("/mark-all-read", HandleMarkAllNotificationsRead)
	notifications.Delete("/:id", HandleDeleteNotification)
	notifications.Delete("/clear-read", HandleClearReadNotifications)

	// ========================================
	// User Management Routes
	// ========================================
	users = app.Group("/admin/users", AuthMiddleware("moderator"))
	users.Get("", HandleUsers)
	users.Get("/table", HandleUsersTable)
	users.Post("/:username/ban", HandleUserBan)
	users.Post("/:username/unban", HandleUserUnban)
	users.Post("/:username/promote", HandleUserPromote)
	users.Post("/:username/demote", HandleUserDemote)

	// ========================================
	// Banned IPs Management
	// ========================================
	bannedIPs = app.Group("/admin/banned-ips", AuthMiddleware("moderator"))
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
	permissions = app.Group("/api/permissions", AuthMiddleware("moderator"))
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
	libraries = app.Group("/admin/libraries", AuthMiddleware("admin"))
	libraries.Get("", HandleLibraries)
	libraries.Post("", HandleCreateLibrary)
	libraries.Get("/:slug", HandleEditLibrary)
	libraries.Put("/:slug", HandleUpdateLibrary)
	libraries.Delete("/:slug", HandleDeleteLibrary)
	libraries.Post("/:slug/toggle", HandleToggleLibrary)
	libraries.Post("/:slug/scan", HandleScanLibrary)
	libraries.Get("/:slug/logs", HandleIndexerLogsWebSocketUpgrade)
	libraries.Post("/:slug/index-posters", HandleIndexPosters)
	libraries.Post("/:slug/index-metadata", HandleIndexMetadata)
	libraries.Post("/:slug/index-chapters", HandleIndexChapters)
	libraries.Get("/helpers/add-folder", HandleAddFolder)
	libraries.Get("/helpers/remove-folder", HandleRemoveFolder)
	libraries.Get("/helpers/cancel-edit", HandleCancelEdit)
	libraries.Get("/helpers/browse", HandleBrowseDirectory)

	// ========================================
	// Scraper Routes
	// ========================================
	scraper = app.Group("/admin/scraper", AuthMiddleware("admin"))
	scraper.Get("", HandleScraper)
	scraper.Get("/new", HandleScraperNewForm)
	scraper.Post("", HandleScraperScriptCreate)
	scraper.Get("/:id", HandleScraperScriptDetail)
	scraper.Get("/:id/logs/view", HandleScraperLogs)
	scraper.Put("/:id", HandleScraperScriptUpdate)
	scraper.Delete("/:id", HandleScraperScriptDelete)
	scraper.Delete("/:id/logs/:logId", HandleScraperLogDelete)
	scraper.Post("/:id/run", HandleScraperScriptRun)
	scraper.Post("/:id/disable", HandleScraperScriptDisable)
	scraper.Post("/:id/enable", HandleScraperScriptEnable)
	scraper.Post("/:id/cancel", HandleScraperScriptCancel)
	scraper.Get("/:id/logs", HandleScraperLogsWebSocketUpgrade)

	scraperHelpers := app.Group("/admin/scraper/helpers", AuthMiddleware("moderator"))
	scraperHelpers.Get("/add-variable", HandleScraperVariableAdd)
	scraperHelpers.Get("/remove-variable", HandleScraperVariableRemove)
	scraperHelpers.Get("/update-script-path", HandleScraperUpdateScriptPath)
	scraperHelpers.Get("/cancel-edit", HandleScraperCancelEdit)

	// ========================================
	// Issue Management
	// ========================================
	app.Get("/admin/issues", AuthMiddleware("admin"), HandleIssuesAdmin)
	apiAdmin := app.Group("/admin", AuthMiddleware("admin"))
	apiAdmin.Put("/issues/:id/status", HandleUpdateIssueStatus)

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
	backups.Get("", handleBackups)
	backups.Post("/create", handleCreateBackup)
	backups.Post("/restore/:filename", handleRestoreBackup)

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
	enablePrefork := true
	log.Fatal(app.Listen(":"+port, fiber.ListenConfig{EnablePrefork: enablePrefork}))
}
