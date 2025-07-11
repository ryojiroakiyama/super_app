package handler

import (
	"bufio"
	"io"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"gmail-tts-app/internal/domain/message"
	"gmail-tts-app/internal/domain/tts"
	ucmsg "gmail-tts-app/internal/usecase/message"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// MessageHandler bundles dependencies for message-related HTTP routes.
type MessageHandler struct {
	uc    *ucmsg.GenerateAudioFromMessage
	repo  message.Repository
	synth tts.Synthesizer
}

func NewMessageHandler(uc *ucmsg.GenerateAudioFromMessage, repo message.Repository, synth tts.Synthesizer) *MessageHandler {
	return &MessageHandler{uc: uc, repo: repo, synth: synth}
}

// Register registers routes to app.
func (h *MessageHandler) Register(app *fiber.App) {
	app.Post("/messages/:id/tts", h.generateAudio)
	app.Get("/messages/:id/tts/stream", h.streamAudio)
}

func (h *MessageHandler) generateAudio(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "id required")
	}
	log.Printf("[handler] generateAudio id=%s", id)
	limitChars := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limitChars = n
		}
	}
	out, err := h.uc.Execute(c.Context(), &ucmsg.GenerateAudioFromMessageInput{
		MessageID:  id,
		LimitChars: limitChars,
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	log.Printf("[handler] generateAudio done path=%s size=%d", out.LocalPath, len(out.AudioBase64)/4*3)
	return c.JSON(out)
}

func (h *MessageHandler) streamAudio(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "id required")
	}
	log.Printf("[handler] streamAudio id=%s", id)
	limitChars := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limitChars = n
		}
	}

	// 1. fetch message body
	msg, err := h.repo.GetByID(c.Context(), message.ID(id))
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}
	text := msg.Body
	if limitChars > 0 && utf8.RuneCountInString(text) > limitChars {
		runeSlice := []rune(text)[:limitChars]
		text = string(runeSlice)
	}

	parts := splitByRuneCount(text, 3000)

	ctx := c.Context()
	c.Context().SetContentType("audio/mpeg")
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		for i, part := range parts {
			rc, err := h.synth.SynthesizeStream(ctx, part)
			if err != nil {
				w.WriteString(err.Error())
				w.Flush()
				return
			}
			io.Copy(w, rc)
			rc.Close()
			w.Flush()
			if i < len(parts)-1 {
				time.Sleep(200 * time.Millisecond)
			}
		}
	})
	return nil
}

// splitByRuneCount splits s into chunks each having at most n runes.
func splitByRuneCount(s string, n int) []string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
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
