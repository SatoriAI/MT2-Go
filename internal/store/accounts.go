package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SatoriAI/MT2-Go/internal/account"
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
	return a.one(ctx, `WHERE lower(name) = lower($1)`, name)
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

func (a Accounts) ChooseEmpire(ctx context.Context, id int64, empire account.Empire) (bool, error) {
	tag, err := a.Pool.Exec(ctx, `UPDATE accounts SET empire = $2 WHERE id = $1 AND empire = 0`, id, empire)
	return tag.RowsAffected() == 1, err
}

func (a Accounts) Characters(ctx context.Context, accountID int64) ([]account.Character, error) {
	rows, err := a.Pool.Query(ctx, `
		SELECT id, slot, name, class, sex, level, play_minutes, st, ht, dx, iq,
		       hair, map, x, y, skill_group
		FROM characters WHERE account_id = $1 ORDER BY slot`, accountID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (account.Character, error) {
		var c account.Character
		err := row.Scan(&c.ID, &c.Slot, &c.Name, &c.Class, &c.Sex, &c.Level, &c.PlayMinutes,
			&c.ST, &c.HT, &c.DX, &c.IQ, &c.Hair, &c.Map, &c.X, &c.Y, &c.SkillGroup)
		return c, err
	})
}
