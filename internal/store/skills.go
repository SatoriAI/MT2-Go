package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Skills keeps each character's skill group and the skills it has learnt.
type Skills struct {
	Pool *pgxpool.Pool
}

// Learnt returns the character's skills, each at its level.
func (s Skills) Learnt(ctx context.Context, characterID uint32) (map[account.Skill]uint8, error) {
	rows, err := s.Pool.Query(ctx, `SELECT skill, level FROM skills WHERE character_id = $1`, characterID)
	if err != nil {
		return nil, err
	}
	learnt := map[account.Skill]uint8{}
	var skill account.Skill
	var level uint8
	_, err = pgx.ForEachRow(rows, []any{&skill, &level}, func() error {
		learnt[skill] = level
		return nil
	})
	return learnt, err
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

// Raise keeps the skill one level higher, learning it at 1, and returns its level now. The
// database refuses a level past 20.
func (s Skills) Raise(ctx context.Context, characterID uint32, skill account.Skill) (uint8, error) {
	var level uint8
	err := s.Pool.QueryRow(ctx, `INSERT INTO skills (character_id, skill, level) VALUES ($1, $2, 1)
		ON CONFLICT (character_id, skill) DO UPDATE SET level = skills.level + 1 RETURNING level`,
		characterID, skill).Scan(&level)
	return level, err
}
