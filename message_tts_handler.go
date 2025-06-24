package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/gmail/v1"
)

func registerMessageTTSEndpoint(app *fiber.App) {
	app.Post("/messages/:id/tts", messageToTTSHandler)
}

func messageToTTSHandler(c *fiber.Ctx) error {
	msgID := c.Params("id")
	if msgID == "" {
		return fiber.NewError(http.StatusBadRequest, "message id required")
	}

	// Build gmail service
	tok, err := tokenFromFile()
	if err != nil {
		return fiber.NewError(http.StatusUnauthorized, "authorize first via /auth/google")
	}
	ga, err := newGoogleAuth()
	if err != nil {
		return fiber.ErrInternalServerError
	}
	srv, err := gmailServiceFromToken(context.Background(), tok, ga.config)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	// Fetch full message
	gm, err := srv.Users.Messages.Get("me", msgID).Format("full").Do()
	if err != nil {
		log.Printf("failed to get message: %v", err)
		return fiber.ErrInternalServerError
	}

	bodyText := extractPlainText(gm.Payload)
	if bodyText == "" {
		// fallback to html converted to text (rudimentary strip tags) or snippet
		if gm.Snippet != "" {
			bodyText = gm.Snippet
		} else if htmlText := extractHTML(gm.Payload); htmlText != "" {
			bodyText = stripHTML(htmlText)
		}
	}
	if bodyText == "" {
		return fiber.NewError(fiber.StatusNotFound, "no text content found")
	}

	// Synthesize speech via OpenAI
	audioBytes, err := synthesizeOpenAITTS(bodyText)
	if err != nil {
		log.Printf("tts error: %v", err)
		return fiber.ErrInternalServerError
	}

	// Save to local file
	if err := os.MkdirAll("audios", 0o755); err != nil {
		return fiber.ErrInternalServerError
	}
	filePath := filepath.Join("audios", fmt.Sprintf("%s.mp3", msgID))
	if err := os.WriteFile(filePath, audioBytes, 0o644); err != nil {
		return fiber.ErrInternalServerError
	}

	b64 := base64.StdEncoding.EncodeToString(audioBytes)

	return c.JSON(fiber.Map{
		"id":          msgID,
		"localPath":   filePath,
		"audioBase64": b64,
	})
}

// extractPlainText traverses message parts to find text/plain content and returns decoded string.
func extractPlainText(p *gmail.MessagePart) string {
	if p == nil {
		return ""
	}
	if p.MimeType == "text/plain" && p.Body != nil && p.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(p.Body.Data)
		if err == nil {
			return string(data)
		}
	}
	for _, part := range p.Parts {
		if txt := extractPlainText(part); txt != "" {
			return txt
		}
	}
	return ""
}

// synthesizeOpenAITTS calls OpenAI TTS and returns MP3 bytes.
func synthesizeOpenAITTS(text string) ([]byte, error) {
	apiKey := getOpenAIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("openai key missing")
	}
	payload := map[string]interface{}{
		"model":  "tts-1",
		"input":  text,
		"voice":  "alloy",
		"format": "mp3",
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return io.ReadAll(resp.Body)
}

// extractHTML finds text/html part content
func extractHTML(p *gmail.MessagePart) string {
	if p == nil {
		return ""
	}
	if p.MimeType == "text/html" && p.Body != nil && p.Body.Data != "" {
		if data, err := base64.URLEncoding.DecodeString(p.Body.Data); err == nil {
			return string(data)
		}
	}
	for _, part := range p.Parts {
		if h := extractHTML(part); h != "" {
			return h
		}
	}
	return ""
}

// very naive html stripper
func stripHTML(s string) string {
	inTag := false
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}
