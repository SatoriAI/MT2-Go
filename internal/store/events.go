package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SatoriAI/MT2-Go/internal/event"
)

// Events keeps the scheduled events.
type Events struct {
	Pool *pgxpool.Pool
}

// exclusionViolation is PostgreSQL's code for a row an exclusion constraint refuses.
const exclusionViolation = "23P01"

// Create stores an event and returns its id; one whose time overlaps another's is
// refused with event.ErrOverlap.
func (s Events) Create(ctx context.Context, e event.Event) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO events (name, exp_percent, yang_percent, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`, e.Name, e.ExpPercent, e.YangPercent, e.Start, e.End).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == exclusionViolation {
		return 0, event.ErrOverlap
	}
	return id, err
}

// List returns every event, past and to come, by when it starts.
func (s Events) List(ctx context.Context) ([]event.Event, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, name, exp_percent, yang_percent, starts_at, ends_at FROM events ORDER BY starts_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanEvent)
}

// Delete removes an event, and reports whether there was one with the id. A running event
// deleted ends at the server's next look.
func (s Events) Delete(ctx context.Context, id int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, id)
	return tag.RowsAffected() == 1, err
}

// Running returns the event running at now, if any.
func (s Events) Running(ctx context.Context, now time.Time) (event.Event, bool, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, name, exp_percent, yang_percent, starts_at, ends_at FROM events
		WHERE tstzrange(starts_at, ends_at) @> $1::timestamptz`, now)
	if err != nil {
		return event.Event{}, false, err
	}
	e, err := pgx.CollectExactlyOneRow(rows, scanEvent)
	if errors.Is(err, pgx.ErrNoRows) {
		return event.Event{}, false, nil
	}
	if err != nil {
		return event.Event{}, false, fmt.Errorf("reading the running event: %w", err)
	}
	return e, true, nil
}

func scanEvent(row pgx.CollectableRow) (event.Event, error) {
	var e event.Event
	err := row.Scan(&e.ID, &e.Name, &e.ExpPercent, &e.YangPercent, &e.Start, &e.End)
	return e, err
}
