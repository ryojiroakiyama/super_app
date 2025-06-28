package tts

import (
	"context"
	"io"
)

// Audio is raw synthesized voice.
type Audio struct {
	Data   []byte
	Format string // e.g. "mp3"
}

// Synthesizer converts text to Audio.
// Concrete implementation wraps OpenAI, Google, Azure, etc.
type Synthesizer interface {
	// Synthesize takes text and returns Audio.
	Synthesize(ctx context.Context, text string) (*Audio, error)

	// SynthesizeStream streams MP3 bytes as they are generated.
	// Caller must Close the returned ReadCloser.
	SynthesizeStream(ctx context.Context, text string) (io.ReadCloser, error)
}
