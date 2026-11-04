package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Whispers keeps every whisper delivered between characters.
type Whispers struct {
	Pool *pgxpool.Pool
}

// Save stores a whisper sender delivered to recipient.
func (w Whispers) Save(ctx context.Context, sender, recipient uint32, text string) error {
	_, err := w.Pool.Exec(ctx, `INSERT INTO whispers (sender, recipient, text) VALUES ($1, $2, $3)`,
		sender, recipient, text)
	return err
}
