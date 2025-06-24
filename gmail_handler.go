package main

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// messageSummary is light representation returned to client.
type messageSummary struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Snippet string `json:"snippet"`
}

// registerGmailRoutes sets up /messages endpoint.
func registerGmailRoutes(app *fiber.App) {
	app.Get("/messages", listMessagesHandler)
}

func listMessagesHandler(c *fiber.Ctx) error {
	// Parse max query param (default 10)
	max := int64(10)
	if m := c.Query("max"); m != "" {
		if v, err := strconv.ParseInt(m, 10, 64); err == nil && v > 0 {
			max = v
		}
	}

	// Load OAuth2 token
	tok, err := tokenFromFile()
	if err != nil {
		return fiber.NewError(http.StatusUnauthorized, "missing token, authorize first via /auth/google")
	}

	// Build auth config (reuse credentials)
	ga, err := newGoogleAuth()
	if err != nil {
		log.Printf("failed to build google auth: %v", err)
		return fiber.ErrInternalServerError
	}

	// Create gmail service
	srv, err := gmailServiceFromToken(context.Background(), tok, ga.config)
	if err != nil {
		log.Printf("unable to retrieve Gmail client: %v", err)
		return fiber.ErrInternalServerError
	}

	// Call Gmail messages list
	user := "me"
	r, err := srv.Users.Messages.List(user).MaxResults(max).LabelIds("INBOX").Do()
	if err != nil {
		log.Printf("unable to retrieve messages: %v", err)
		return fiber.ErrInternalServerError
	}

	var summaries []messageSummary

	for _, m := range r.Messages {
		msg, err := srv.Users.Messages.Get(user, m.Id).Format("metadata").MetadataHeaders("Subject").Do()
		if err != nil {
			log.Printf("failed to get message %s: %v", m.Id, err)
			continue
		}
		subj := "(no subject)"
		for _, h := range msg.Payload.Headers {
			if h.Name == "Subject" {
				subj = h.Value
				break
			}
		}
		summaries = append(summaries, messageSummary{
			ID:      m.Id,
			Subject: subj,
			Snippet: msg.Snippet,
		})
	}

	return c.JSON(fiber.Map{"messages": summaries})
}
