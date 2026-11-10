package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// Buy takes price yang from the character and keeps what was bought, both or neither: into
// is a stack of its kind at its new count, or, with no id, a new item. It returns the item
// kept, with its id, and the yang left. The database refuses a price the character cannot
// pay.
func (s Items) Buy(ctx context.Context, characterID, price uint32, into item.Item) (item.Item, uint32, error) {
	var gold uint32
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `UPDATE characters SET gold = gold - $2 WHERE id = $1 RETURNING gold`,
			characterID, price).Scan(&gold); err != nil {
			return err
		}
		if into.ID != 0 {
			return setCount(ctx, tx, characterID, into.ID, into.Count)
		}
		return tx.QueryRow(ctx, `INSERT INTO items (character_id, vnum, count, cell) VALUES ($1, $2, $3, $4)
			RETURNING id`, characterID, into.VNum, into.Count, into.Cell).Scan(&into.ID)
	})
	return into, gold, err
}

// Sell gives the character pay yang for what was sold, and keeps what is left of the item,
// both or neither: left at its lower count, or forgotten with none left. It returns the
// yang the character now has. The database refuses more than a character may hold.
func (s Items) Sell(ctx context.Context, characterID, pay uint32, left item.Item) (uint32, error) {
	var gold uint32
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `UPDATE characters SET gold = gold + $2 WHERE id = $1 RETURNING gold`,
			characterID, pay).Scan(&gold); err != nil {
			return err
		}
		return keepLeft(ctx, tx, characterID, left)
	})
	return gold, err
}
