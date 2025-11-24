package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/executor"
	"github.com/alexander-bruun/magi/indexer"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	
)

// savedCacheDirectory stores the cache directory path for image downloads
var savedCacheDirectory string

// Initialize configures all HTTP routes, middleware, and static assets for the application
func Initialize(app *fiber.App, cacheDirectory string, port string) {
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
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	app.Use(OptionalAuthMiddleware())
	app.Use(healthcheck.New())

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
		return handleImageRequest(c, savedCacheDirectory)
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
	media.Get("/:media<[A-Za-z0-9_-]+>/metadata/form", AuthMiddleware("moderator"), HandleUpdateMetadataMedia)
	media.Post("/:media<[A-Za-z0-9_-]+>/metadata/manual", AuthMiddleware("moderator"), HandleManualEditMetadata)
	media.Post("/:media<[A-Za-z0-9_-]+>/metadata/refresh", AuthMiddleware("moderator"), HandleRefreshMetadata)
	media.Post("/:media<[A-Za-z0-9_-]+>/metadata/overwrite", AuthMiddleware("moderator"), HandleEditMetadataMedia)
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

// handleImageRequest serves images with quality based on user role
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

	// Get user role for compression quality
	userName, _ := c.Locals("user_name").(string)
	var quality int
	if userName != "" {
		user, err := models.FindUserByUsername(userName)
		if err == nil && user != nil {
			quality = models.GetCompressionQualityForRole(user.Role)
		} else {
			quality = models.GetCompressionQualityForRole("reader") // default for authenticated but error
		}
	} else {
		quality = models.GetCompressionQualityForRole("anonymous") // default for anonymous
	}

	// Load the image
	file, err := os.Open(fullPath)
	if err != nil {
		// If loading fails, serve original file
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
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		// If loading fails, serve original file
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

	// Encode all images as JPEG for better performance and consistent compression
	var buf bytes.Buffer
	// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
	jpegQuality := quality
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	c.Set("Content-Type", "image/jpeg")

	if err != nil {
		// If encoding fails, serve original file
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

	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.Send(buf.Bytes())
}

// ImageHandler serves images for both comics and light novels using token-based authentication
func ImageHandler(c *fiber.Ctx) error {
	log.Debugf("ImageHandler called with token: %s", c.Query("token"))
	
	token := c.Query("token")
	
	if token == "" {
		return handleErrorWithStatus(c, fmt.Errorf("token parameter is required"), fiber.StatusBadRequest)
	}

	// Validate and consume the token
	tokenInfo, err := utils.ValidateAndConsumeImageToken(token)
	if err != nil {
		log.Errorf("Token validation failed for token %s: %v", token, err)
		return handleErrorWithStatus(c, fmt.Errorf("invalid or expired token: %w", err), fiber.StatusForbidden)
	}

	// Validate MediaSlug to prevent malformed tokens
	if strings.ContainsAny(tokenInfo.MediaSlug, "/,") {
		log.Errorf("Invalid MediaSlug in token: %s", tokenInfo.MediaSlug)
		return handleErrorWithStatus(c, fmt.Errorf("invalid token"), fiber.StatusForbidden)
	}

	media, err := models.GetMedia(tokenInfo.MediaSlug)
	if err != nil {
		log.Errorf("Failed to get media %s: %v", tokenInfo.MediaSlug, err)
		return handleError(c, err)
	}
	if media == nil {
		log.Errorf("Media not found for slug: %s, %v, %v", tokenInfo.MediaSlug, tokenInfo.ChapterSlug, tokenInfo.AssetPath)
		return handleErrorWithStatus(c, fmt.Errorf("media not found or access restricted based on content rating settings"), fiber.StatusNotFound)
	}
	
	var chapter *models.Chapter
	if tokenInfo.AssetPath != "" {
		// Light novel asset: chapterSlug may be 0 or empty, but should be valid for asset lookup
		chapterSlug := tokenInfo.ChapterSlug
		if chapterSlug == "" || strings.ContainsAny(chapterSlug, "./ ") {
			// Fallback: try to extract from Referer if possible
			referer := c.Get("Referer")
			if referer != "" {
				parts := strings.Split(referer, "/series/")
				if len(parts) > 1 {
					slugParts := strings.Split(parts[1], "/")
					if len(slugParts) > 1 {
						chapterSlug = slugParts[1]
					}
				}
			}
		}
		chapter, err = models.GetChapter(tokenInfo.MediaSlug, chapterSlug)
		if err != nil {
			log.Errorf("Failed to get chapter %s/%s: %v", tokenInfo.MediaSlug, chapterSlug, err)
			return handleError(c, err)
		}
		if chapter == nil {
			return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
		}
		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			return handleError(c, err)
		}
		if !hasAccess {
			return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
		}
		return serveLightNovelAsset(c, media, chapter, tokenInfo.AssetPath)
	} else {
		// Comic page
		chapter, err = models.GetChapter(tokenInfo.MediaSlug, tokenInfo.ChapterSlug)
		if err != nil {
			log.Errorf("Failed to get chapter %s/%s: %v", tokenInfo.MediaSlug, tokenInfo.ChapterSlug, err)
			return handleError(c, err)
		}
		if chapter == nil {
			return handleErrorWithStatus(c, fmt.Errorf("chapter not found"), fiber.StatusNotFound)
		}
		hasAccess, err := UserHasLibraryAccess(c, chapter.MediaSlug)
		if err != nil {
			return handleError(c, err)
		}
		if !hasAccess {
			return handleErrorWithStatus(c, fmt.Errorf("access denied: you don't have permission to view this chapter"), fiber.StatusForbidden)
		}
		return serveComicPage(c, media, chapter, tokenInfo.Page)
	}
}

// serveComicPage serves a comic page image
func serveComicPage(c *fiber.Ctx, media *models.Media, chapter *models.Chapter, page int) error {
	// Determine the actual chapter file path
	// For single-file media (cbz/cbr), media.Path is the file itself
	// For directory-based media, we need to join path and chapter file
	filePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		filePath = filepath.Join(media.Path, chapter.File)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return handleErrorWithStatus(c, fmt.Errorf("file not found"), fiber.StatusNotFound)
	}

	// If the path is a directory, serve images from within it
	if fileInfo.IsDir() {
		return serveImageFromDirectory(c, filePath, page)
	}

	lowerFileName := strings.ToLower(fileInfo.Name())

	// Serve the file based on its extension
	switch {
	case strings.HasSuffix(lowerFileName, ".jpg"), strings.HasSuffix(lowerFileName, ".jpeg"),
		strings.HasSuffix(lowerFileName, ".png"), strings.HasSuffix(lowerFileName, ".webp"),
		strings.HasSuffix(lowerFileName, ".gif"):
		return c.SendFile(filePath)
	case strings.HasSuffix(lowerFileName, ".cbr"), strings.HasSuffix(lowerFileName, ".rar"):
		return serveComicBookArchiveFromRAR(c, filePath, page)
	case strings.HasSuffix(lowerFileName, ".cbz"), strings.HasSuffix(lowerFileName, ".zip"):
		return serveComicBookArchiveFromZIP(c, filePath, page)
	default:
		return handleErrorWithStatus(c, fmt.Errorf("unsupported file type"), fiber.StatusBadRequest)
	}
}

// serveLightNovelAsset serves a light novel asset from an EPUB file
func serveLightNovelAsset(c *fiber.Ctx, media *models.Media, chapter *models.Chapter, assetPath string) error {
	// Determine the actual chapter file path
	chapterFilePath := media.Path
	if fileInfo, err := os.Stat(media.Path); err == nil && fileInfo.IsDir() {
		chapterFilePath = filepath.Join(media.Path, chapter.File)
	}

	log.Debugf("Light novel asset request: media=%s, chapter=%s, asset=%s, file=%s", media.Slug, chapter.Slug, assetPath, chapterFilePath)

	log.Debugf("Serving asset: %s\n", assetPath)

	// Check if the file exists
	if _, err := os.Stat(chapterFilePath); os.IsNotExist(err) {
		log.Errorf("EPUB file not found: %s", chapterFilePath)
		return handleErrorWithStatus(c, fmt.Errorf("EPUB file not found"), fiber.StatusNotFound)
	}

	// Open the EPUB file
	r, err := zip.OpenReader(chapterFilePath)
	if err != nil {
		log.Errorf("Error opening EPUB %s: %v", chapterFilePath, err)
		return handleErrorWithStatus(c, fmt.Errorf("error opening EPUB: %w", err), fiber.StatusInternalServerError)
	}
	defer r.Close()

	// Get the OPF directory
	opfDir, err := utils.GetOPFDir(chapterFilePath)
	if err != nil {
		log.Errorf("Error getting OPF dir for %s: %v", chapterFilePath, err)
		return handleErrorWithStatus(c, fmt.Errorf("error parsing EPUB: %w", err), fiber.StatusInternalServerError)
	}

	log.Debugf("OPF dir: %s, requested asset: %s", opfDir, assetPath)

	// Find the asset
	assetFullPath := filepath.Join(opfDir, assetPath)
	log.Debugf("Looking for asset at: %s", assetFullPath)
	
	var file *zip.File
	for _, f := range r.File {
		log.Debugf("EPUB file: %s", f.Name)
		if f.Name == assetFullPath {
			file = f
			break
		}
	}
	if file == nil {
		log.Errorf("Asset not found in EPUB: %s (looked for %s)", assetPath, assetFullPath)
		return handleErrorWithStatus(c, fmt.Errorf("asset not found"), fiber.StatusNotFound)
	}

	log.Debugf("Asset found in EPUB: %s", assetFullPath)

	rc, err := file.Open()
	if err != nil {
		log.Errorf("Error opening asset %s: %v", assetPath, err)
		return handleErrorWithStatus(c, fmt.Errorf("error opening asset: %w", err), fiber.StatusInternalServerError)
	}
	defer rc.Close()

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(assetPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Set("Content-Type", "image/jpeg")
	case ".png":
		c.Set("Content-Type", "image/png")
	case ".gif":
		c.Set("Content-Type", "image/gif")
	case ".svg":
		c.Set("Content-Type", "image/svg+xml")
	case ".css":
		c.Set("Content-Type", "text/css")
	case ".xhtml", ".html":
		c.Set("Content-Type", "text/html")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}

	if _, err := io.Copy(c.Response().BodyWriter(), rc); err != nil {
		log.Errorf("Error writing asset %s to response: %v", assetPath, err)
		return handleErrorWithStatus(c, fmt.Errorf("error writing asset: %w", err), fiber.StatusInternalServerError)
	}

	return nil
}
