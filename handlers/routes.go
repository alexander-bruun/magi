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

	// Admin endpoints
	app.Get("/admin", HandleAdmin)
	app.Post("/admin/create-library", HandleCreateLibrary)
	app.Delete("/admin/delete-library/:id", HandleDeleteLibrary)
	app.Put("/admin/update-library/:id", HandleUpdateLibrary)
	app.Get("/admin/edit-library/:id", HandleEditLibrary)
	app.Get("/admin/add-folder", HandleAddFolder)
	app.Get("/admin/remove-folder", HandleRemoveFolder)
	app.Get("/admin/cancel-edit", HandleCancelEdit)

	// Manga endpoints
	app.Get("/mangas", HandleMangas)
	app.Get("/metadata-form/:id", HandleUpdateMetadataManga)
	app.Post("/overwrite-metadata", HandleEditMetadataManga)
	app.Get("/manga-search", HandleMangaSearch)
	app.Get("/:manga", HandleManga)
	app.Get("/:manga/:chapter", HandleChapter)

	// Fallback
	app.Get("/*", HandleNotFound)

	log.Fatal(app.Listen(":3000"))
}
