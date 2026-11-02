package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Accounts keeps accounts and their characters in PostgreSQL.
type Accounts struct {
	Pool *pgxpool.Pool
}

// CreateAccount stores a new account and returns its id.
func (a Accounts) CreateAccount(ctx context.Context, name string, passwordHash []byte) (int64, error) {
	var id int64
	err := a.Pool.QueryRow(ctx,
		`INSERT INTO accounts (name, password_hash) VALUES ($1, $2) RETURNING id`,
		name, passwordHash).Scan(&id)
	return id, err
}

// AccountByName ignores case: names are unique regardless of it.
func (a Accounts) AccountByName(ctx context.Context, name string) (account.Account, error) {
	return a.one(ctx, `WHERE lower(name) = lower($1)`, name)
}

func (a Accounts) AccountByID(ctx context.Context, id int64) (account.Account, error) {
	return a.one(ctx, `WHERE id = $1`, id)
}

// one reads an account. A kingdom not chosen yet is NULL in the table and "" in Go.
func (a Accounts) one(ctx context.Context, where string, arg any) (account.Account, error) {
	var acc account.Account
	err := a.Pool.QueryRow(ctx,
		`SELECT id, name, password_hash, coalesce(empire, '') FROM accounts `+where, arg).
		Scan(&acc.ID, &acc.Name, &acc.PasswordHash, &acc.Empire)
	if errors.Is(err, pgx.ErrNoRows) {
		return account.Account{}, account.ErrNotFound
	}
	return acc, err
}

// ChooseEmpire sets the kingdom of an account that has none, and reports whether the
// account now has that kingdom: choosing the same one again succeeds, another one fails.
func (a Accounts) ChooseEmpire(ctx context.Context, id int64, empire account.Empire) (bool, error) {
	tag, err := a.Pool.Exec(ctx,
		`UPDATE accounts SET empire = $2 WHERE id = $1 AND (empire IS NULL OR empire = $2)`, id, empire)
	return tag.RowsAffected() == 1, err
}

func (a Accounts) Characters(ctx context.Context, accountID int64) ([]account.Character, error) {
	rows, err := a.Pool.Query(ctx, `
		SELECT id, slot, name, class, sex, level, play_minutes, st, ht, dx, iq,
		       hair, x, y, coalesce(skill_group, '')
		FROM characters WHERE account_id = $1 ORDER BY slot`, accountID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (account.Character, error) {
		var c account.Character
		err := row.Scan(&c.ID, &c.Slot, &c.Name, &c.Class, &c.Sex, &c.Level, &c.PlayMinutes,
			&c.ST, &c.HT, &c.DX, &c.IQ, &c.Hair, &c.X, &c.Y, &c.SkillGroup)
		return c, err
	})
}
