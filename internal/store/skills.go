package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
)

// Skills keeps each character's skill group and the skills it has learnt.
type Skills struct {
	Pool *pgxpool.Pool
}

// Learnt returns the character's skills, each at its level, and when those trained by a
// book or a soul stone may next be.
func (s Skills) Learnt(ctx context.Context, characterID uint32) (map[account.Skill]uint8, map[account.Skill]time.Time, error) {
	rows, err := s.Pool.Query(ctx, `SELECT skill, level, next_read FROM skills WHERE character_id = $1`, characterID)
	if err != nil {
		return nil, nil, err
	}
	learnt, reads := map[account.Skill]uint8{}, map[account.Skill]time.Time{}
	var skill account.Skill
	var level uint8
	var next *time.Time
	_, err = pgx.ForEachRow(rows, []any{&skill, &level, &next}, func() error {
		learnt[skill] = level
		if next != nil {
			reads[skill] = *next
		}
		return nil
	})
	return learnt, reads, err
}

// Choose sets the character's skill group and forgets every skill it had learnt, both or
// neither, so a change of group gives every point back.
func (s Skills) Choose(ctx context.Context, characterID uint32, group account.SkillGroup) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE characters SET skill_group = $2 WHERE id = $1`, characterID, group); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM skills WHERE character_id = $1`, characterID)
		return err
	})
}

// Raise keeps the skill at the level a point raised it to, learning it at the first.
func (s Skills) Raise(ctx context.Context, characterID uint32, skill account.Skill, level uint8) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO skills (character_id, skill, level) VALUES ($1, $2, $3)
		ON CONFLICT (character_id, skill) DO UPDATE SET level = excluded.level`, characterID, skill, level)
	return err
}

// ErrNotTrained means a training found the skill no longer at the level it was read at, or
// the book or stone no longer the character's in its bag.
var ErrNotTrained = errors.New("store: the skill or what trains it changed")

// Train keeps a read of a book or a soul stone, all of it or none: the skill from level
// from at level to, the same on a failed read, and next read no sooner than next; and
// what trained it used, the stack at its count left, or forgotten with none left.
func (s Skills) Train(ctx context.Context, characterID uint32, skill account.Skill, from, to uint8, next time.Time, used item.Item) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE skills SET level = $4, next_read = $5
			WHERE character_id = $1 AND skill = $2 AND level = $3`, characterID, skill, from, to, next)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrNotTrained
		}
		if used.Count == 0 {
			tag, err = tx.Exec(ctx, `DELETE FROM items WHERE id = $1 AND character_id = $2 AND worn IS NULL`, used.ID, characterID)
		} else {
			tag, err = tx.Exec(ctx, `UPDATE items SET count = $3 WHERE id = $1 AND character_id = $2 AND worn IS NULL`,
				used.ID, characterID, used.Count)
		}
		if err == nil && tag.RowsAffected() != 1 {
			err = ErrNotTrained
		}
		return err
	})
}
