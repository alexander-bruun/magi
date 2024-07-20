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

	// Comic handler
	app.Get("/api/comic", ComicHandler)

	// Static assets and images
	log.Info(cacheDirectory)
	app.Static("/api/images", cacheDirectory)
	app.Static("/assets/", "./assets/")

	// Register views
	app.Get("/", HandleHome)
	app.Get("/mangas", HandleMangas)
	app.Get("/manga/:slug", HandleManga)
	app.Get("/libraries", HandleLibraries)
	app.Get("/library/:slug", HandleLibrary)
	app.Get("/admin", HandleAdmin)

	// Register library handlers
	app.Get("/api/libraries", HandleGetLibraries)
	app.Post("/api/libraries", HandleCreateLibrary)
	app.Put("/api/libraries", HandleUpdateLibrary)
	app.Get("/api/libraries/:id", HandleGetLibrary)
	app.Delete("/api/libraries/:id", HandleDeleteLibrary)

	// New HTMX-specific routes for libraries
	app.Get("/api/add-folder", HandleAddFolder)
	app.Get("/api/remove-folder", HandleRemoveFolder)

	// Fallback
	app.Use(HandleNotFound)

	log.Fatal(app.Listen(":3000"))
}
