package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Whispers keeps every whisper delivered between characters.
type Whispers struct {
	Pool *pgxpool.Pool
}

// Save stores a whisper sender delivered to recipient at sentAt.
func (w Whispers) Save(ctx context.Context, sender, recipient uint32, text string, sentAt time.Time) error {
	_, err := w.Pool.Exec(ctx, `INSERT INTO whispers (sender, recipient, text, sent_at) VALUES ($1, $2, $3, $4)`,
		sender, recipient, text, sentAt)
	return err
}
