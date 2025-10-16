package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
)

// Local export of the cache directory, so the image download function knows where to store the cached images.
var savedCacheDirectory string

// Initialize wires up all HTTP routes, middleware, and static assets for the Fiber app.
func Initialize(app *fiber.App, cacheDirectory string) {
	log.Info("Initializing GoFiber view routes")

	savedCacheDirectory = cacheDirectory

	// CORS middleware configuration to allow all origins
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	// Optional auth: populate c.Locals("user_name") when cookies are set so pages
	// can show personalized UI without forcing login.
	app.Use(OptionalAuthMiddleware())

	// Handle preflight requests for CORS
	app.Options("/*", func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		return c.SendStatus(fiber.StatusOK)
	})

	app.Use(healthcheck.New())

	// - .zip (implemented)
	// - .cbz (implemented)
	// - .rar (implemented)
	// - .cbr (implemented)
	// - .pdf
	// - .jpg (implemented)
	// - .png (implemented)
	// - .mobi
	// - .epub
	// Any other file type is blocked.
	app.Get("/api/comic", ComicHandler)

	// Static assets and images
	app.Static("/api/images", cacheDirectory)
	app.Static("/assets/", "./assets/")

	// Register views
	app.Get("/", HandleHome)
	app.Get("/login", LoginHandler)
	app.Get("/register", RegisterHandler)
	app.Post("/register", CreateUserHandler)
	app.Post("/login", LoginUserHandler)
	app.Post("/logout", LogoutHandler)

	// Libraries endpoint group
	libraries := app.Group("/libraries", AuthMiddleware("admin"))

	// CRUD endpoints
	libraries.Get("", HandleLibraries)
	libraries.Post("", HandleCreateLibrary)
	libraries.Delete("/:slug", HandleDeleteLibrary)
	libraries.Put("/:slug", HandleUpdateLibrary)

	// Form endpoints
	libraries.Get("/edit-library/:slug", HandleEditLibrary)
	libraries.Get("/scan/:slug", HandleScanLibrary)
	libraries.Get("/add-folder", HandleAddFolder)
	libraries.Get("/remove-folder", HandleRemoveFolder)
	libraries.Get("/cancel-edit", HandleCancelEdit)
	
	// Better page - duplicate detection
	libraries.Get("/better", HandleBetter)

	// Users endpoint group
	users := app.Group("/users", AuthMiddleware("moderator"))

	// CRUD endpoints
	users.Get("", HandleUsers)
	users.Get("/ban/:username", HandleUserBan)
	users.Get("/unban/:username", HandleUserUnban)
	users.Get("/promote/:username", HandleUserPromote)
	users.Get("/demote/:username", HandleUserDemote)

	// Configuration page (admin only)
	app.Get("/configuration", AuthMiddleware("admin"), HandleConfiguration)
	app.Post("/configuration", AuthMiddleware("admin"), HandleConfigurationUpdate)

	// Better page (admin only) - duplicate detection
	app.Get("/better", AuthMiddleware("admin"), HandleBetter)
	
	// API endpoints
	api := app.Group("/api", AuthMiddleware("admin"))
	api.Post("/duplicates/:id/dismiss", HandleDismissDuplicate)
	api.Get("/duplicates/:id/folder-info", HandleGetDuplicateFolderInfo)
	api.Delete("/duplicates/:id/folder", HandleDeleteDuplicateFolder)

	// Manga endpoint group
	mangas := app.Group("/mangas")
	mangas.Get("", HandleMangas)
	mangas.Get("/tags", HandleTags)
	mangas.Get("/tags-fragment", HandleTagsFragment)
	mangas.Get("/metadata-form/:slug", HandleUpdateMetadataManga)
	mangas.Post("/overwrite-metadata", HandleEditMetadataManga)
	mangas.Post("/:manga/manual-edit-metadata", AuthMiddleware("moderator"), HandleManualEditMetadata)
	mangas.Post("/:manga/refresh-metadata", AuthMiddleware("moderator"), HandleRefreshMetadata)
	mangas.Get("/search", HandleMangaSearch)
	mangas.Get("/:manga", HandleManga)
	// Voting endpoints (HTMX) - register before the chapter wildcard so they match first
	mangas.Post("/:manga/vote", AuthMiddleware("reader"), HandleMangaVote)
	mangas.Get("/:manga/vote-fragment", HandleMangaVoteFragment)
	// Favorite endpoints (HTMX)
	mangas.Post("/:manga/favorite", AuthMiddleware("reader"), HandleMangaFavorite)
	mangas.Get("/:manga/favorite-fragment", HandleMangaFavoriteFragment)
	mangas.Get("/:manga/:chapter", HandleChapter)
	// Reading state endpoints (HTMX)
	mangas.Post("/:manga/:chapter/read", AuthMiddleware("reader"), HandleMarkRead)
	mangas.Post("/:manga/:chapter/unread", AuthMiddleware("reader"), HandleMarkUnread)

	// Account page for authenticated users
	app.Get("/account", AuthMiddleware("reader"), HandleAccount)

	// Account paginated lists
	app.Get("/account/favorites", AuthMiddleware("reader"), HandleAccountFavorites)
	app.Get("/account/upvoted", AuthMiddleware("reader"), HandleAccountUpvoted)
	app.Get("/account/downvoted", AuthMiddleware("reader"), HandleAccountDownvoted)
	app.Get("/account/reading", AuthMiddleware("reader"), HandleAccountReading)

	// Fallback
	app.Get("/*", HandleNotFound)

	log.Fatal(app.Listen(":3000"))
}
