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
	"strconv"
	"strings"
	"time"

	"bufio"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/gmail/v1"
)

func registerMessageTTSEndpoint(app *fiber.App) {
	app.Post("/messages/:id/tts", messageToTTSHandler)
	app.Get("/messages/:id/tts/stream", messageToTTSStreamHandler)
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

	// 1. 抜き出せる限りのテキストを組み立て
	bodyText := collectMessageText(gm)
	if limStr := c.Query("limit"); limStr != "" {
		if v, err := strconv.Atoi(limStr); err == nil && v > 0 {
			bodyText = truncateRunes(bodyText, v)
		}
	}
	if len(bodyText) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "no text content found")
	}

	// 2. OpenAI TTS は 4096 文字制限のためチャンク分割 (余裕を見て 3,000 文字)
	const maxChars = 3000
	chunks := splitByRuneCount(bodyText, maxChars)
	var combined []byte
	for idx, ck := range chunks {
		audio, err := synthesizeOpenAITTS(ck)
		if err != nil {
			log.Printf("tts error (chunk %d/%d): %v", idx+1, len(chunks), err)
			return fiber.NewError(http.StatusBadGateway, err.Error())
		}
		combined = append(combined, audio...)
	}
	audioBytes := combined

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

// messageToTTSStreamHandler streams synthesized MP3 back to client using chunked transfer encoding.
func messageToTTSStreamHandler(c *fiber.Ctx) error {
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

	// Build text
	bodyText := collectMessageText(gm)
	if limStr := c.Query("limit"); limStr != "" {
		if v, err := strconv.Atoi(limStr); err == nil && v > 0 {
			bodyText = truncateRunes(bodyText, v)
		}
	}
	if len(bodyText) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "no text content found")
	}

	// 分割（OpenAI 制限）
	const maxChars = 3000
	chunks := splitByRuneCount(bodyText, maxChars)

	// ヘッダ設定
	c.Set("Content-Type", "audio/mpeg")

	// BodyStreamWriter: チャンクごとに OpenAI へストリーム転送しコピー
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		for idx, ck := range chunks {
			rc, err := openAITTSStream(ck)
			if err != nil {
				log.Printf("openai stream error (chunk %d/%d): %v", idx+1, len(chunks), err)
				return
			}
			_, _ = io.Copy(w, rc)
			w.Flush()
			rc.Close()
		}
	})

	return nil
}

// openAITTSStream calls OpenAI with stream=true and returns the response body (caller must Close).
func openAITTSStream(text string) (io.ReadCloser, error) {
	apiKey := getOpenAIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("openai key missing")
	}
	payload := map[string]interface{}{
		"model":  "tts-1",
		"input":  text,
		"voice":  "alloy",
		"format": "mp3",
		"stream": true,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// 長時間ストリームの可能性があるためタイムアウト無しのクライアントを使用
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return resp.Body, nil
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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

// collectMessageText tries to build a readable text from the Gmail message.
// 1) すべての text/plain パートを連結
// 2) 連結テキストが短い場合は HTML → テキスト変換
// 3) それでも短ければ Snippet を返す
func collectMessageText(msg *gmail.Message) string {
	if msg == nil || msg.Payload == nil {
		return ""
	}

	// Gather all text/plain parts
	var plainParts []string
	gatherPlainText(msg.Payload, &plainParts)
	plainText := strings.TrimSpace(strings.Join(plainParts, "\n"))

	// If plain text is sufficiently long, use it
	if len([]rune(plainText)) >= 300 { // heuristic
		return plainText
	}

	// Otherwise try HTML -> text
	if html := extractHTML(msg.Payload); html != "" {
		txt := stripHTML(html)
		if len([]rune(txt)) > len([]rune(plainText)) {
			return txt
		}
	}

	if plainText != "" {
		return plainText
	}
	return msg.Snippet
}

// gatherPlainText appends all decoded text/plain contents into the slice.
func gatherPlainText(p *gmail.MessagePart, out *[]string) {
	if p == nil {
		return
	}
	if p.MimeType == "text/plain" && p.Body != nil && p.Body.Data != "" {
		if data, err := base64.URLEncoding.DecodeString(p.Body.Data); err == nil {
			*out = append(*out, string(data))
		}
	}
	for _, part := range p.Parts {
		gatherPlainText(part, out)
	}
}

// splitByRuneCount splits a string into chunks where each chunk has at most max rune length.
func splitByRuneCount(s string, max int) []string {
	if max <= 0 {
		return []string{s}
	}
	var chunks []string
	runeSlice := []rune(s)
	for start := 0; start < len(runeSlice); start += max {
		end := start + max
		if end > len(runeSlice) {
			end = len(runeSlice)
		}
		chunks = append(chunks, string(runeSlice[start:end]))
	}
	return chunks
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
