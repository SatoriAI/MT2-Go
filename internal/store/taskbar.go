package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Taskbar keeps what each of a character's taskbar slots holds: an item, by its id, or a
// skill.
type Taskbar struct {
	Pool *pgxpool.Pool
}

// Slots returns the character's filled slots, each with what it holds.
func (s Taskbar) Slots(ctx context.Context, characterID uint32) (map[uint8]account.Shortcut, error) {
	rows, err := s.Pool.Query(ctx, `SELECT slot, item_id, skill FROM taskbar WHERE character_id = $1`, characterID)
	if err != nil {
		return nil, err
	}
	return slotsOf(rows)
}

// slotsOf reads rows of a slot and what it holds.
func slotsOf(rows pgx.Rows) (map[uint8]account.Shortcut, error) {
	slots := map[uint8]account.Shortcut{}
	var slot uint8
	var item *int64
	var skill *string
	_, err := pgx.ForEachRow(rows, []any{&slot, &item, &skill}, func() error {
		var held account.Shortcut
		if item != nil {
			held.Item = *item
		}
		if skill != nil {
			held.Skill = account.Skill(*skill)
		}
		slots[slot] = held
		return nil
	})
	return slots, err
}

// Set fills the slot with an item or a skill, in place of whatever it held.
func (s Taskbar) Set(ctx context.Context, characterID uint32, slot uint8, held account.Shortcut) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO taskbar (character_id, slot, item_id, skill) VALUES ($1, $2, $3, $4)
		ON CONFLICT (character_id, slot) DO UPDATE SET item_id = excluded.item_id, skill = excluded.skill`,
		characterID, slot, nullItem(held.Item), nullSkill(held.Skill))
	return err
}

func nullItem(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func nullSkill(s account.Skill) *string {
	if s == "" {
		return nil
	}
	name := string(s)
	return &name
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
			RETURNING slot, item_id, skill`, characterID, a, b)
		if err != nil {
			return err
		}
		held, err := slotsOf(rows)
		if err != nil {
			return err
		}
		for from, h := range held {
			to := a
			if from == a {
				to = b
			}
			if _, err := tx.Exec(ctx, `INSERT INTO taskbar (character_id, slot, item_id, skill) VALUES ($1, $2, $3, $4)`,
				characterID, to, nullItem(h.Item), nullSkill(h.Skill)); err != nil {
				return err
			}
		}
		return nil
	})
}
