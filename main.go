package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

func main() {
	app := fiber.New()

	// Healthcheck endpoint
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	log.Println("🚀  Server listening on http://localhost:8080")
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
