package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// Trade carries out a trade in one transaction, and returns the yang each side then has.
// An item a side no longer owns, or carries worn, refuses all of it with ErrNotOwned, and
// the database refuses a side's yang going below nothing or past what it may hold. A slot of
// the taskbar holding an item given goes with it, and the trade is kept in the log.
func (s Items) Trade(ctx context.Context, t item.Trade) ([2]uint32, error) {
	var gold [2]uint32
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var tradeID int64
		if err := tx.QueryRow(ctx, `INSERT INTO trades (first_id, second_id, first_gold, second_gold)
			VALUES ($1, $2, $3, $4) RETURNING id`, t[0].Character, t[1].Character, t[0].Gold, t[1].Gold).Scan(&tradeID); err != nil {
			return err
		}
		for i, side := range t {
			to := t[1-i].Character
			for _, it := range side.Items {
				if err := owned(tx.Exec(ctx, `UPDATE items SET character_id = $3, cell = $4
					WHERE id = $1 AND character_id = $2 AND worn IS NULL`, it.ID, side.Character, to, it.Cell)); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `DELETE FROM taskbar WHERE item_id = $1`, it.ID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO traded_items (trade_id, item_id, vnum, count, from_id)
					VALUES ($1, $2, $3, $4, $5)`, tradeID, it.ID, it.VNum, it.Count, side.Character); err != nil {
					return err
				}
			}
		}
		for i, side := range t {
			if err := tx.QueryRow(ctx, `UPDATE characters SET gold = gold - $2 + $3 WHERE id = $1 RETURNING gold`,
				side.Character, side.Gold, t[1-i].Gold).Scan(&gold[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return gold, err
}
