package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Accounts implements account.Store on PostgreSQL.
type Accounts struct {
	Pool *pgxpool.Pool
}

var _ account.Store = Accounts{}

// CreateAccount stores a new account and returns its id.
func (a Accounts) CreateAccount(ctx context.Context, name string, passwordHash []byte) (int64, error) {
	var id int64
	err := a.Pool.QueryRow(ctx,
		`INSERT INTO accounts (name, password_hash) VALUES ($1, $2) RETURNING id`,
		name, passwordHash).Scan(&id)
	return id, err
}

func (a Accounts) AccountByName(ctx context.Context, name string) (account.Account, error) {
	return a.one(ctx, `WHERE name = $1`, name)
}

func (a Accounts) AccountByID(ctx context.Context, id int64) (account.Account, error) {
	return a.one(ctx, `WHERE id = $1`, id)
}

func (a Accounts) one(ctx context.Context, where string, arg any) (account.Account, error) {
	var acc account.Account
	err := a.Pool.QueryRow(ctx,
		`SELECT id, name, password_hash, empire FROM accounts `+where, arg).
		Scan(&acc.ID, &acc.Name, &acc.PasswordHash, &acc.Empire)
	if errors.Is(err, pgx.ErrNoRows) {
		return account.Account{}, account.ErrNotFound
	}
	return acc, err
}

func (a Accounts) SetEmpire(ctx context.Context, id int64, empire protocol.Empire) error {
	tag, err := a.Pool.Exec(ctx, `UPDATE accounts SET empire = $2 WHERE id = $1`, id, empire)
	if err == nil && tag.RowsAffected() == 0 {
		return account.ErrNotFound
	}
	return err
}

func (a Accounts) Characters(ctx context.Context, accountID int64) ([protocol.CharacterSlots]protocol.CharacterSummary, error) {
	var slots [protocol.CharacterSlots]protocol.CharacterSummary
	rows, err := a.Pool.Query(ctx, `
		SELECT slot, id, name, job, level, play_minutes, st, ht, dx, iq,
		       body_part, hair_part, x, y, skill_group
		FROM characters WHERE account_id = $1`, accountID)
	if err != nil {
		return slots, err
	}
	defer rows.Close()
	for rows.Next() {
		var slot int
		var c protocol.CharacterSummary
		if err := rows.Scan(&slot, &c.ID, &c.Name, &c.Job, &c.Level, &c.PlayMinutes,
			&c.ST, &c.HT, &c.DX, &c.IQ, &c.BodyPart, &c.HairPart, &c.X, &c.Y, &c.SkillGroup); err != nil {
			return slots, err
		}
		slots[slot] = c
	}
	return slots, rows.Err()
}
