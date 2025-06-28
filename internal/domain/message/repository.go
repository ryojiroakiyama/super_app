package message

import "context"

// Repository abstracts data source of EmailMessage.
// The implementation will live in infrastructure layer (e.g., Gmail API).
// This keeps the domain independent of external services.
type Repository interface {
	GetByID(ctx context.Context, id ID) (*EmailMessage, error)
}
