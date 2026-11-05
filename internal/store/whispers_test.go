package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// The time of saving is not the time of sending, so a whisper stored without the time it
// was sent is refused rather than given one.
func TestAWhisperIsNotStoredWithoutTheTimeItWasSent(t *testing.T) {
	accounts, _ := accountInShinsoo(t)

	_, err := accounts.Pool.Exec(context.Background(),
		`INSERT INTO whispers (sender, recipient, text) VALUES (0, 0, 'hi')`)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23502" || pgErr.ColumnName != "sent_at" {
		t.Fatalf("got %v, want sent_at refused as null", err)
	}
}
