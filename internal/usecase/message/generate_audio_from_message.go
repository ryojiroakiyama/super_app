package message

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gmail-tts-app/internal/domain/audio"
	domainmsg "gmail-tts-app/internal/domain/message"
	"gmail-tts-app/internal/domain/tts"
)

// GenerateAudioFromMessageInput is input DTO.
type GenerateAudioFromMessageInput struct {
	MessageID  string
	LimitChars int // 0 means no limit
}

// GenerateAudioFromMessageOutput is output DTO.
type GenerateAudioFromMessageOutput struct {
	ID          string     `json:"id"`
	LocalPath   string     `json:"localPath"`
	AudioBase64 string     `json:"audioBase64"`
	Audio       *tts.Audio `json:"-"` // raw audio (optional)
}

// GenerateAudioFromMessage implements usecase.UseCase.
type GenerateAudioFromMessage struct {
	repo        domainmsg.Repository
	synthesizer tts.Synthesizer
	store       audio.Store
}

func NewGenerateAudioFromMessage(repo domainmsg.Repository, synth tts.Synthesizer, store audio.Store) *GenerateAudioFromMessage {
	return &GenerateAudioFromMessage{repo: repo, synthesizer: synth, store: store}
}

// Execute converts message body to audio and save via store.
func (uc *GenerateAudioFromMessage) Execute(ctx context.Context, in *GenerateAudioFromMessageInput) (*GenerateAudioFromMessageOutput, error) {
	// 1. Fetch message
	msg, err := uc.repo.GetByID(ctx, domainmsg.ID(in.MessageID))
	if err != nil {
		return nil, err
	}

	text := msg.Body
	if in.LimitChars > 0 && runeCount(text) > in.LimitChars {
		text = truncateRunes(text, in.LimitChars)
	}

	// Create safe filename from email subject
	fileName := sanitizeFilename(msg.Subject)
	if fileName == "" {
		fileName = string(msg.ID) // fallback to ID if subject is empty or invalid
	}

	// Split into chunks (<=1500 runes)
	const chunkSize = 1500
	chunks := splitByRuneCount(text, chunkSize)

	var merged []byte
	for i, part := range chunks {
		log.Printf("[uc] synthesize part %d/%d runes=%d", i+1, len(chunks), len([]rune(part)))
		partCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		audioObj, err := uc.synthesizer.Synthesize(partCtx, part)
		cancel()
		if err != nil {
			log.Printf("[uc] synthesize error on part %d: %v", i+1, err)
			return nil, err
		}
		// Save individual chunk for debugging
		partFile := fmt.Sprintf("parts/%s_part%d", fileName, i+1)
		if _, err := uc.store.Save(audioObj.Data, partFile); err != nil {
			log.Printf("[uc] save part error: %v", err)
			return nil, err
		}
		merged = append(merged, audioObj.Data...)
	}

	log.Printf("[uc] all parts synthesized, total bytes=%d", len(merged))

	// Save merged audio
	mergedPath, err := uc.store.Save(merged, filepath.Join("merged", fileName))
	if err != nil {
		return nil, err
	}

	b64 := base64.StdEncoding.EncodeToString(merged)

	return &GenerateAudioFromMessageOutput{
		ID:          string(msg.ID),
		LocalPath:   string(mergedPath),
		AudioBase64: b64,
		Audio:       &tts.Audio{Data: merged, Format: "mp3"},
	}, nil
}

// sanitizeFilename converts email subject to a safe filename by:
// 1. Removing/replacing invalid characters for file systems
// 2. Limiting length to reasonable size
// 3. Trimming whitespace
func sanitizeFilename(subject string) string {
	if subject == "" {
		return ""
	}

	// Replace invalid characters with underscore
	// Invalid chars: / \ : * ? " < > | and control characters
	invalidChars := regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f\x7f]`)
	filename := invalidChars.ReplaceAllString(subject, "_")

	// Replace multiple consecutive underscores with single underscore
	multipleUnderscores := regexp.MustCompile(`_+`)
	filename = multipleUnderscores.ReplaceAllString(filename, "_")

	// Trim whitespace and underscores
	filename = strings.Trim(filename, " _")

	// Limit length to 100 characters (reasonable for most file systems)
	if len(filename) > 100 {
		runes := []rune(filename)
		if len(runes) > 100 {
			filename = string(runes[:100])
		}
	}

	// Ensure it doesn't end with dot or space (Windows compatibility)
	filename = strings.TrimRight(filename, ". ")

	return filename
}

// Helper functions (copied from existing code for now). Eventually move to pkg/util
func runeCount(s string) int {
	return len([]rune(s))
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if n > len(r) {
		return s
	}
	return string(r[:n])
}

// splitByRuneCount splits string s into chunks each having at most n runes.
func splitByRuneCount(s string, n int) []string {
	if n <= 0 || runeCount(s) <= n {
		return []string{s}
	}
	var res []string
	var builder strings.Builder
	count := 0
	for _, r := range s {
		builder.WriteRune(r)
		count++
		if count >= n {
			res = append(res, builder.String())
			builder.Reset()
			count = 0
		}
	}
	if builder.Len() > 0 {
		res = append(res, builder.String())
	}
	return res
}
