package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"gmail-tts-app/internal/infrastructure/googleauth"

	"github.com/gofiber/fiber/v2"
	gmailv1 "google.golang.org/api/gmail/v1"
)

// messageSummary is light representation returned to client.
type messageSummary struct {
	ID           string `json:"id"`
	Subject      string `json:"subject"`
	Snippet      string `json:"snippet"`
	InternalDate int64  `json:"internalDate"`
	From         string `json:"from"`
	Preview      string `json:"preview"`
}

// RegisterGmailListRoutes sets up /messages endpoints for listing.
func RegisterGmailListRoutes(app *fiber.App) {
	app.Get("/messages", listMessagesHandler)
	app.Get("/messages/latest", latestMessageHandler)
}

func listMessagesHandler(c *fiber.Ctx) error {
	// Parse max query param (default 5)
	max := int64(5)
	if m := c.Query("max"); m != "" {
		if v, err := strconv.ParseInt(m, 10, 64); err == nil && v > 0 {
			max = v
		}
	}

	// Optional Gmail search query
	q := c.Query("q")

	log.Printf("[handler] list messages: max=%d q=%s", max, q)

	// Load OAuth2 token
	tok, err := googleauth.TokenFromFile()
	if err != nil {
		return fiber.NewError(http.StatusUnauthorized, "missing token, authorize first via /auth/google")
	}

	// Build auth config (reuse credentials)
	ga, err := googleauth.NewGoogleAuth()
	if err != nil {
		log.Printf("failed to build google auth: %v", err)
		return fiber.ErrInternalServerError
	}

	// Create gmail service
	srv, err := googleauth.GmailServiceFromToken(context.Background(), tok, ga.Config())
	if err != nil {
		log.Printf("unable to retrieve Gmail client: %v", err)
		return fiber.ErrInternalServerError
	}

	// Call Gmail messages list
	summaries, err := fetchMessageSummaries(srv, max, q)
	if err != nil {
		log.Printf("failed fetch messages: %v", err)
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"messages": summaries})
}

func latestMessageHandler(c *fiber.Ctx) error {
	// Reuse list handler logic with max=1
	c.Request().URI().QueryArgs().Set("max", "1")
	return listMessagesHandler(c)
}

func fetchMessageSummaries(srv *gmailv1.Service, max int64, q string) ([]messageSummary, error) {
	user := "me"
	listCall := srv.Users.Messages.List(user).MaxResults(max).LabelIds("INBOX")
	if q != "" {
		listCall = listCall.Q(q)
	}

	r, err := listCall.Do()
	if err != nil {
		return nil, err
	}

	var summaries []messageSummary
	for _, m := range r.Messages {
		msg, err := srv.Users.Messages.Get(user, m.Id).Format("metadata").MetadataHeaders("Subject", "From").Do()
		if err != nil {
			log.Printf("failed to get message %s: %v", m.Id, err)
			continue
		}
		subj := "(no subject)"
		from := ""
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "Subject":
				subj = h.Value
			case "From":
				from = h.Value
			}
		}
		preview := msg.Snippet
		if runeCount := len([]rune(preview)); runeCount > 20 {
			preview = string([]rune(preview)[:20])
		}
		summaries = append(summaries, messageSummary{
			ID:           m.Id,
			Subject:      subj,
			Snippet:      preview,
			InternalDate: msg.InternalDate,
			From:         from,
			Preview:      preview,
		})
	}
	return summaries, nil
}
