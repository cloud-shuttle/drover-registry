package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	app := fiber.New(fiber.Config{
		AppName: "drover-registry v0.1.0",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())

	// Health & ready endpoints
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "drover-registry",
		})
	})
	app.Get("/readyz", func(c *fiber.Ctx) error {
		// TODO: check DB, storage connectivity
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// TODO: v1 API routes for packages, versions, search
	// app.Post("/v1/packages", handlers.PublishPackage)
	// app.Get("/v1/packages/:name", handlers.GetPackage)
	// etc.

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 drover-registry listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
