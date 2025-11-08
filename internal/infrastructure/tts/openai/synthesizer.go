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

    "gmail-tts-app/internal/config"
    "gmail-tts-app/internal/domain/tts"
)

// Synthesizer implements tts.Synthesizer using OpenAI TTS endpoint.
type Synthesizer struct {
	apiKey         string
	voice          string
	model          string
	speed          float64
	responseFormat string
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

	// Load TTS configuration from tts.config file
	ttsConfig, err := config.LoadTTSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load TTS config: %w", err)
	}

	return &Synthesizer{
		apiKey:         apiKey,
		voice:          ttsConfig.Voice,
		model:          ttsConfig.Model,
		speed:          ttsConfig.Speed,
		responseFormat: ttsConfig.ResponseFormat,
	}, nil
}

// Synthesize converts text to audio bytes (mp3).
func (s *Synthesizer) Synthesize(ctx context.Context, text string) (*tts.Audio, error) {
	payload := map[string]interface{}{
		"model":           s.model,
		"input":           text,
		"voice":           s.voice,
		"speed":           s.speed,
		"response_format": s.responseFormat,
	}
	body, _ := json.Marshal(payload)

	// Log TTS execution start
	textLen := len([]rune(text))
	fmt.Printf("[tts] Starting OpenAI TTS synthesis...\n")
	fmt.Printf("[tts]   - Model: %s\n", s.model)
	fmt.Printf("[tts]   - Voice: %s\n", s.voice)
	fmt.Printf("[tts]   - Speed: %.1f\n", s.speed)
	fmt.Printf("[tts]   - Format: %s\n", s.responseFormat)
	fmt.Printf("[tts]   - Text length: %d characters\n", textLen)
	fmt.Printf("[tts] Sending request to OpenAI API...\n")

	startTime := time.Now()

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
	
	fmt.Printf("[tts] Response received, reading audio data...\n")
	
	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	duration := time.Since(startTime)
	fmt.Printf("[tts] TTS synthesis completed successfully!\n")
	fmt.Printf("[tts]   - Audio size: %d bytes (%.2f KB)\n", len(audioBytes), float64(len(audioBytes))/1024)
	fmt.Printf("[tts]   - Processing time: %.2f seconds\n", duration.Seconds())
	
	return &tts.Audio{Data: audioBytes, Format: "mp3"}, nil
}
// Stream synth is unused in CLI mode and intentionally omitted.

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
