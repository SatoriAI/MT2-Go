package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Taskbar keeps what each of a character's taskbar slots holds: an item, by its id.
type Taskbar struct {
	Pool *pgxpool.Pool
}

// Slots returns the character's filled slots, each with the item it holds.
func (s Taskbar) Slots(ctx context.Context, characterID uint32) (map[uint8]int64, error) {
	rows, err := s.Pool.Query(ctx, `SELECT slot, item_id FROM taskbar WHERE character_id = $1`, characterID)
	if err != nil {
		return nil, err
	}
	return slotsOf(rows)
}

// slotsOf reads rows of a slot and the item it holds.
func slotsOf(rows pgx.Rows) (map[uint8]int64, error) {
	slots := map[uint8]int64{}
	var slot uint8
	var item int64
	_, err := pgx.ForEachRow(rows, []any{&slot, &item}, func() error {
		slots[slot] = item
		return nil
	})
	return slots, err
}

// Set fills the slot with the item, in place of whatever it held.
func (s Taskbar) Set(ctx context.Context, characterID uint32, slot uint8, itemID int64) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO taskbar (character_id, slot, item_id) VALUES ($1, $2, $3)
		ON CONFLICT (character_id, slot) DO UPDATE SET item_id = excluded.item_id`, characterID, slot, itemID)
	return err
}

// Clear empties the slot.
func (s Taskbar) Clear(ctx context.Context, characterID uint32, slot uint8) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM taskbar WHERE character_id = $1 AND slot = $2`, characterID, slot)
	return err
}

// Swap swaps what the two slots hold, either of which may be empty, both or neither.
func (s Taskbar) Swap(ctx context.Context, characterID uint32, a, b uint8) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `DELETE FROM taskbar WHERE character_id = $1 AND slot IN ($2, $3)
			RETURNING slot, item_id`, characterID, a, b)
		if err != nil {
			return err
		}
		held, err := slotsOf(rows)
		if err != nil {
			return err
		}
		for from, item := range held {
			to := a
			if from == a {
				to = b
			}
			if _, err := tx.Exec(ctx, `INSERT INTO taskbar (character_id, slot, item_id) VALUES ($1, $2, $3)`,
				characterID, to, item); err != nil {
				return err
			}
		}
		return nil
	})
}
