package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
)

func Initialize(app *fiber.App, cacheDirectory string) {
	log.Info("Initializing GoFiber routes!")

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
	log.Info(cacheDirectory)
	app.Static("/api/images", cacheDirectory)
	app.Static("/assets/", "./assets/")

	app.Get("/", HandleHome)
	app.Get("/mangas", HandleMangas)
	app.Get("/manga/:slug", HandleManga)
	app.Get("/libraries", HandleLibraries)
	app.Get("/library/:slug", HandleLibrary)

	// Register library handlers
	app.Get("/api/libraries", GetLibrariesHandler)
	app.Post("/api/libraries", CreateLibraryHandler)
	app.Put("/api/libraries", UpdateLibraryHandler)
	app.Get("/api/libraries/:id", GetLibraryHandler)
	app.Delete("/api/libraries/:id", DeleteLibraryHandler)
	app.Get("/api/libraries/search", SearchLibrariesHandler)

	// Register manga handlers
	app.Get("/api/mangas", GetMangasHandler)
	app.Post("/api/mangas", CreateMangaHandler)
	app.Get("/api/mangas/:id", GetMangaHandler)
	app.Put("/api/mangas/:id", UpdateMangaHandler)
	app.Delete("/api/mangas/:id", DeleteMangaHandler)

	// Register chapter handlers
	app.Post("/api/chapters", CreateChapterHandler)
	app.Get("/api/chapters/:id", GetChapterHandler)
	app.Put("/api/chapters/:id", UpdateChapterHandler)
	app.Delete("/api/chapters/:id", DeleteChapterHandler)
	app.Get("/api/chapters/search", SearchChaptersHandler)

	// Fallback
	app.Get("/*", HandleNotFound)

	log.Fatal(app.Listen(":3000"))
}
