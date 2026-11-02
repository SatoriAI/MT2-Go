package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// ChooseEmpire sets the account's kingdom while it has no character, and reports whether
// the account now has that kingdom. Once a character exists, only the same kingdom again
// succeeds.
func (a Accounts) ChooseEmpire(ctx context.Context, id int64, empire account.Empire) (bool, error) {
	var chosen bool
	err := a.withAccount(ctx, id, func(tx pgx.Tx, current account.Empire) error {
		if current == empire {
			chosen = true
			return nil
		}
		var hasCharacters bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM characters WHERE account_id = $1)`, id).
			Scan(&hasCharacters); err != nil || hasCharacters {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE accounts SET empire = $2 WHERE id = $1`, id, empire); err != nil {
			return err
		}
		chosen = true
		return nil
	})
	return chosen, err
}

// CreateCharacter stores c for the account and returns its id. It refuses with
// account.ErrKingdomChanged unless the account's kingdom is still empire, the one the
// character's start position was taken from.
func (a Accounts) CreateCharacter(ctx context.Context, accountID int64, empire account.Empire, c account.Character) (uint32, error) {
	var id uint32
	err := a.withAccount(ctx, accountID, func(tx pgx.Tx, current account.Empire) error {
		if current != empire {
			return account.ErrKingdomChanged
		}
		return tx.QueryRow(ctx, `
			INSERT INTO characters (account_id, slot, name, class, sex, shape,
			                        vitality, intelligence, strength, dexterity, x, y)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
			accountID, c.Slot, c.Name, c.Class, c.Sex, c.Shape,
			c.Vitality, c.Intelligence, c.Strength, c.Dexterity, c.X, c.Y).Scan(&id)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "characters_name_key":
			return 0, account.ErrNameTaken
		case "characters_account_id_slot_key":
			return 0, account.ErrSlotTaken
		}
	}
	return id, err
}

// withAccount runs fn in a transaction holding the account's row lock, so whatever
// depends on its kingdom and its characters cannot interleave. Each statement in fn sees
// what the transactions before it committed. NO KEY UPDATE is the weakest lock that
// conflicts with itself; it leaves inserts that only reference the account unblocked.
func (a Accounts) withAccount(ctx context.Context, id int64, fn func(pgx.Tx, account.Empire) error) error {
	return pgx.BeginFunc(ctx, a.Pool, func(tx pgx.Tx) error {
		var empire account.Empire
		if err := tx.QueryRow(ctx, `SELECT coalesce(empire, '') FROM accounts WHERE id = $1 FOR NO KEY UPDATE`, id).
			Scan(&empire); err != nil {
			return err
		}
		return fn(tx, empire)
	})
}

// characterColumns are what account.Character holds, in the order scanCharacter reads them.
const characterColumns = `id, slot, name, class, sex, shape, level, play_minutes,
	vitality, intelligence, strength, dexterity, hair, x, y, coalesce(skill_group, '')`

func scanCharacter(row pgx.Row) (account.Character, error) {
	var c account.Character
	err := row.Scan(&c.ID, &c.Slot, &c.Name, &c.Class, &c.Sex, &c.Shape, &c.Level, &c.PlayMinutes,
		&c.Vitality, &c.Intelligence, &c.Strength, &c.Dexterity, &c.Hair, &c.X, &c.Y, &c.SkillGroup)
	return c, err
}

func (a Accounts) Characters(ctx context.Context, accountID int64) ([]account.Character, error) {
	rows, err := a.Pool.Query(ctx,
		`SELECT `+characterColumns+` FROM characters WHERE account_id = $1 ORDER BY slot`, accountID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (account.Character, error) { return scanCharacter(row) })
}

// Character reads the account's character in slot, or returns account.ErrNotFound.
func (a Accounts) Character(ctx context.Context, accountID int64, slot int) (account.Character, error) {
	c, err := scanCharacter(a.Pool.QueryRow(ctx,
		`SELECT `+characterColumns+` FROM characters WHERE account_id = $1 AND slot = $2`, accountID, slot))
	if errors.Is(err, pgx.ErrNoRows) {
		return c, account.ErrNotFound
	}
	return c, err
}

// SavePosition keeps where the character stands for its next login.
func (a Accounts) SavePosition(ctx context.Context, characterID uint32, x, y int32) error {
	_, err := a.Pool.Exec(ctx, `UPDATE characters SET x = $2, y = $3 WHERE id = $1`, characterID, x, y)
	return err
}
