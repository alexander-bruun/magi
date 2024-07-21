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

	// Static assets and images
	log.Info(cacheDirectory)
	app.Static("/api/images", cacheDirectory)
	app.Static("/assets/", "./assets/")

	// Register views
	app.Get("/", HandleHome)

	// Manga endpoints
	app.Get("/mangas", HandleMangas)
	app.Get("/manga/:slug", HandleManga)

	// Library endpoints
	app.Get("/libraries", HandleLibraries)
	app.Get("/library/:slug", HandleLibrary)

	// Admin endpoints
	app.Get("/admin", HandleAdmin)
	app.Post("/admin/create-library", HandleCreateLibrary)
	app.Delete("/admin/delete-library/:id", HandleDeleteLibrary)
	app.Put("/admin/update-library/:id", HandleUpdateLibrary)
	app.Get("/admin/edit-library/:id", HandleEditLibrary)
	app.Get("/admin/add-folder", HandleAddFolder)
	app.Get("/admin/remove-folder", HandleRemoveFolder)
	app.Get("/admin/cancel-edit", HandleCancelEdit)

	// Fallback
	app.Get("/*", HandleNotFound)

	log.Fatal(app.Listen(":3000"))
}
