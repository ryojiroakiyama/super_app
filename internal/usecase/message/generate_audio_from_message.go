package message

import (
	"context"
	"encoding/base64"

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

	// 2. Synthesize
	audioObj, err := uc.synthesizer.Synthesize(ctx, text)
	if err != nil {
		return nil, err
	}

	// 3. Store
	path, err := uc.store.Save(audioObj.Data, string(msg.ID))
	if err != nil {
		return nil, err
	}

	b64 := base64.StdEncoding.EncodeToString(audioObj.Data)

	return &GenerateAudioFromMessageOutput{
		ID:          string(msg.ID),
		LocalPath:   string(path),
		AudioBase64: b64,
		Audio:       audioObj,
	}, nil
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
