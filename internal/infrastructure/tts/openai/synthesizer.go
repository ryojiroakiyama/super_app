package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gmail-tts-app/internal/domain/tts"
)

// Synthesizer implements tts.Synthesizer using OpenAI TTS endpoint.
type Synthesizer struct {
	apiKey string
	voice  string
	model  string
}

// NewSynthesizer creates OpenAI TTS synthesizer.
// If apiKey is empty, it tries environment variable OPENAI_API_KEY or file openai_api_key.txt.
func NewSynthesizer(apiKey string) (*Synthesizer, error) {
	if apiKey == "" {
		apiKey = getOpenAIKey()
	}
	if apiKey == "" {
		return nil, fmt.Errorf("openai api key is required")
	}
	return &Synthesizer{apiKey: apiKey, voice: "alloy", model: "tts-1"}, nil
}

// Synthesize converts text to audio bytes (mp3).
func (s *Synthesizer) Synthesize(ctx context.Context, text string) (*tts.Audio, error) {
	payload := map[string]interface{}{
		"model":  s.model,
		"input":  text,
		"voice":  s.voice,
		"format": "mp3",
	}
	body, _ := json.Marshal(payload)

	// adopt timeout from ctx or fallback to 90s
	reqCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, "POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &tts.Audio{Data: audioBytes, Format: "mp3"}, nil
}

// SynthesizeStream converts text to a stream of audio bytes (mp3).
func (s *Synthesizer) SynthesizeStream(ctx context.Context, text string) (io.ReadCloser, error) {
	reqBody := map[string]any{
		"model":  s.model,
		"voice":  s.voice,
		"input":  text,
		"format": "mp3",
		"stream": true,
	}
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/audio/speech", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai stream status %d: %s", resp.StatusCode, string(raw))
	}
	return resp.Body, nil // caller closes
}

// getOpenAIKey returns the OpenAI API key.
// Priority: env OPENAI_API_KEY > file openai_api_key.txt
func getOpenAIKey() string {
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		return k
	}
	secretsDir := os.Getenv("SECRETS_DIR")
	if secretsDir == "" {
		secretsDir = "secrets"
	}
	path := filepath.Join(secretsDir, "openai_api_key.txt")
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}
