package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ttsRequest payload
type ttsRequest struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
	Voice    string `json:"voice,omitempty"`
}

// registerTTSRoutes registers POST /tts
func registerTTSRoutes(app *fiber.App) {
	app.Post("/tts", ttsHandler)
}

func ttsHandler(c *fiber.Ctx) error {
	var req ttsRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid json")
	}
	if req.Text == "" {
		return fiber.NewError(http.StatusBadRequest, "text is required")
	}

	voice := req.Voice
	if voice == "" {
		voice = "alloy" // default OpenAI voice
	}

	apiKey := getOpenAIKey()
	if apiKey == "" {
		return fiber.NewError(http.StatusInternalServerError, "OpenAI API key not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload := map[string]interface{}{
		"model":  "tts-1",
		"input":  req.Text,
		"voice":  voice,
		"format": "mp3",
	}

	bodyBytes, _ := json.Marshal(payload)

	reqHTTP, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(bodyBytes))
	if err != nil {
		return fiber.ErrInternalServerError
	}
	reqHTTP.Header.Set("Content-Type", "application/json")
	reqHTTP.Header.Set("Authorization", "Bearer "+apiKey)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(reqHTTP)
	if err != nil {
		log.Printf("openai tts request failed: %v", err)
		return fiber.ErrInternalServerError
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		log.Printf("openai tts error status %d: %s", resp.StatusCode, string(b))
		return fiber.ErrInternalServerError
	}

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	b64 := base64.StdEncoding.EncodeToString(audioBytes)
	return c.JSON(fiber.Map{"audioContent": b64})
}

// getOpenAIKey returns the OpenAI API key.
// Priority: 1) environment variable OPENAI_API_KEY 2) file "openai_api_key.txt" (trimmed).
func getOpenAIKey() string {
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		return k
	}
	data, err := os.ReadFile("openai_api_key.txt")
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}
