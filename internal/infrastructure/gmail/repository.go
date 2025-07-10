package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"gmail-tts-app/internal/domain/message"

	"google.golang.org/api/gmail/v1"
)

// MessageRepository implements domain message.Repository backed by Gmail API.
type MessageRepository struct {
	srv *gmail.Service
}

func NewMessageRepository(srv *gmail.Service) *MessageRepository {
	return &MessageRepository{srv: srv}
}

// GetByID fetches Gmail message, aggregates plain text / html to EmailMessage Body.
func (r *MessageRepository) GetByID(ctx context.Context, id message.ID) (*message.EmailMessage, error) {
	log.Printf("[repo] GetByID: %s", id)
	gm, err := r.srv.Users.Messages.Get("me", string(id)).Format("full").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("gmail get message: %w", err)
	}
	body := collectMessageText(gm)
	subj := ""
	for _, h := range gm.Payload.Headers {
		if strings.ToLower(h.Name) == "subject" {
			subj = h.Value
			break
		}
	}
	return &message.EmailMessage{ID: id, Subject: subj, Body: body}, nil
}

// ===== helpers (copied from existing handler) =====

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

// collectMessageText replicates logic from original handler.
func collectMessageText(msg *gmail.Message) string {
	if msg == nil || msg.Payload == nil {
		return ""
	}
	var plainParts []string
	gatherPlainText(msg.Payload, &plainParts)
	plainText := strings.TrimSpace(strings.Join(plainParts, "\n"))

	if len([]rune(plainText)) >= 300 {
		return plainText
	}

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
