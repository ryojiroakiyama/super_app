package main

import (
	"context"
	"log"

	"gmail-tts-app/internal/config"
	gmailrepo "gmail-tts-app/internal/infrastructure/gmail"
	"gmail-tts-app/internal/infrastructure/googleauth"
	"gmail-tts-app/internal/infrastructure/storage"
	"gmail-tts-app/internal/infrastructure/tts/openai"
	"gmail-tts-app/internal/interface/http/handler"
	ucmsg "gmail-tts-app/internal/usecase/message"

	"github.com/gofiber/fiber/v2"
)

func main() {
	cfg := config.Load()

	app := fiber.New()
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// OAuth routes
	if err := googleauth.RegisterOAuthRoutes(app); err != nil {
		log.Fatalf("failed to register OAuth routes: %v", err)
	}

	// Build dependencies
	ctx := context.Background()
	srv, err := googleauth.BuildGmailService(ctx)
	if err != nil {
		log.Fatalf("gmail service build error: %v", err)
	}
	repo := gmailrepo.NewMessageRepository(srv)

	synth, err := openai.NewSynthesizer(cfg.OpenAIAPIKey)
	if err != nil {
		log.Fatalf("openai synth build error: %v", err)
	}
	store := storage.NewFileStore(cfg.AudioDir)

	uc := ucmsg.NewGenerateAudioFromMessage(repo, synth, store)

	mh := handler.NewMessageHandler(uc, repo, synth)
	mh.Register(app)

	// Gmail list routes (legacy)
	handler.RegisterGmailListRoutes(app)

	log.Printf("🚀  Server listening on http://localhost:%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
