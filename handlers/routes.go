package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
)

func Initialize(app *fiber.App, cacheDirectory string) {
	log.Info("Initializing GoFiber view routes")

	// CORS middleware configuration to allow all origins
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

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

	// Admin endpoints
	admin := app.Group("/admin", AuthMiddleware("admin"))
	admin.Get("", HandleAdmin)
	admin.Post("/create-library", HandleCreateLibrary)
	admin.Delete("/delete-library/:id", HandleDeleteLibrary)
	admin.Put("/update-library/:id", HandleUpdateLibrary)
	admin.Get("/edit-library/:id", HandleEditLibrary)
	admin.Get("/add-folder", HandleAddFolder)
	admin.Get("/remove-folder", HandleRemoveFolder)
	admin.Get("/cancel-edit", HandleCancelEdit)

	// Manga endpoints
	mangas := app.Group("/mangas")
	mangas.Get("", HandleMangas)
	mangas.Get("/metadata-form/:id", HandleUpdateMetadataManga)
	mangas.Post("/overwrite-metadata", HandleEditMetadataManga)
	mangas.Get("/search", HandleMangaSearch)
	mangas.Get("/:manga", HandleManga)
	mangas.Get("/:manga/:chapter", HandleChapter)

	// Fallback
	app.Get("/*", HandleNotFound)

	log.Fatal(app.Listen(":3000"))
}
