package main

import (
	"context"
	"log"

	gmailrepo "gmail-tts-app/internal/infrastructure/gmail"
	"gmail-tts-app/internal/infrastructure/storage"
	"gmail-tts-app/internal/infrastructure/tts/openai"
	"gmail-tts-app/internal/interface/http/handler"
	"gmail-tts-app/internal/usecase/message"

	"github.com/gofiber/fiber/v2"
	gmailv1 "google.golang.org/api/gmail/v1"
)

func buildGmailService(ctx context.Context) (*gmailv1.Service, error) {
	tok, err := tokenFromFile()
	if err != nil {
		return nil, err
	}
	ga, err := newGoogleAuth()
	if err != nil {
		return nil, err
	}
	return gmailServiceFromToken(ctx, tok, ga.config)
}

func main() {
	app := fiber.New()

	// Healthcheck endpoint
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Register routes
	if err := RegisterOAuthRoutes(app); err != nil {
		log.Printf("failed to register OAuth routes: %v", err)
	}

	// Gmail list routes (legacy handler)
	registerGmailRoutes(app)

	// Build dependencies
	ctx := context.Background()
	srv, err := buildGmailService(ctx)
	if err != nil {
		log.Fatalf("gmail service build error: %v", err)
	}
	repo := gmailrepo.NewMessageRepository(srv)

	synth, err := openai.NewSynthesizer("")
	if err != nil {
		log.Fatalf("openai synth build error: %v", err)
	}
	store := storage.NewFileStore("audios")

	uc := message.NewGenerateAudioFromMessage(repo, synth, store)

	mh := handler.NewMessageHandler(uc, repo, synth)
	mh.Register(app)

	log.Println("🚀  Server listening on http://localhost:8080")
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
