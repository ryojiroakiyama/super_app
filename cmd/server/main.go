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
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	cfg := config.Load()

	app := fiber.New()
	// CORS middleware for browser access
	app.Use(cors.New())

	// Serve static frontend files
	app.Static("/", "./public")

	// Explicit root index fallback (for safety)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./public/index.html")
	})

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })

    // Placeholders to allow hot re-init after authorization
    var (
        repo  *gmailrepo.MessageRepository
        uc    *ucmsg.GenerateAudioFromMessage
        mh    *handler.MessageHandler
        synth *openai.Synthesizer
        store *storage.FileStore
    )

    // OAuth routes with onAuthorized hook: reload token and rebuild deps
    if err := googleauth.RegisterOAuthRoutes(app, func() {
        ctx := context.Background()
        if srv, err := googleauth.BuildGmailService(ctx); err != nil {
            log.Printf("[auth] rebuild gmail service failed: %v", err)
            return
        } else {
            repo = gmailrepo.NewMessageRepository(srv)
            if store == nil {
                store = storage.NewFileStore(cfg.AudioDir)
            }
            uc = ucmsg.NewGenerateAudioFromMessage(repo, synth, store)
            if mh != nil {
                mh.ReplaceDeps(uc, repo, synth)
                log.Printf("[auth] dependencies re-initialized after authorization")
            } else {
                log.Printf("[auth] message handler not ready; will use new deps on first init")
            }
        }
    }); err != nil {
		log.Fatalf("failed to register OAuth routes: %v", err)
	}

    // Build dependencies
    ctx := context.Background()
    // try initial gmail init

    // Try to build Gmail service, but don't exit if not authorized yet
    if srv, err := googleauth.BuildGmailService(ctx); err != nil {
        log.Printf("[startup] Gmail not authorized or token missing: %v", err)
        log.Printf("[startup] Please authorize via http://localhost:%s/auth/google", cfg.Port)
    } else {
        // Lightweight access check
        if _, err := srv.Users.Labels.List("me").Do(); err != nil {
            log.Printf("[startup] Gmail service reachable but access failed: %v", err)
            log.Printf("[startup] Please re-authorize via http://localhost:%s/auth/google", cfg.Port)
        } else {
            repo = gmailrepo.NewMessageRepository(srv)
            log.Printf("[startup] Gmail authorization OK")
        }
    }

    s, err := openai.NewSynthesizer(cfg.OpenAIAPIKey)
    if err != nil {
        log.Fatalf("openai synth build error: %v", err)
    }
    synth = s
    store = storage.NewFileStore(cfg.AudioDir)

    if repo != nil {
        uc = ucmsg.NewGenerateAudioFromMessage(repo, synth, store)
    }

    mh = handler.NewMessageHandler(uc, repo, synth)
    mh.Register(app)

    // Auth status endpoint for clients/health checks
    app.Get("/auth/status", func(c *fiber.Ctx) error {
        // Quick check: try building service and listing 1 label
        if srv, err := googleauth.BuildGmailService(c.Context()); err != nil {
            return c.JSON(fiber.Map{"authorized": false, "reason": err.Error()})
        } else if _, err := srv.Users.Labels.List("me").Do(); err != nil {
            return c.JSON(fiber.Map{"authorized": false, "reason": err.Error()})
        }
        return c.JSON(fiber.Map{"authorized": true})
    })

	// Gmail list routes (legacy)
	handler.RegisterGmailListRoutes(app)

	log.Printf("🚀  Server listening on http://localhost:%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
