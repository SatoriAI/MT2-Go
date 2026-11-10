package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// Upgrade keeps what an upgrade at the blacksmith did, all of it or none: cost yang taken,
// the item as it now is, its kind upgraded, lowered or kept, and each stack of material
// at what is left of it, forgotten with none left. It returns the yang left. The database
// refuses a cost the character cannot pay.
func (s Items) Upgrade(ctx context.Context, characterID, cost uint32, upgraded item.Item, spent []item.Item) (uint32, error) {
	var gold uint32
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `UPDATE characters SET gold = gold - $2 WHERE id = $1 RETURNING gold`,
			characterID, cost).Scan(&gold); err != nil {
			return err
		}
		if err := owned(tx.Exec(ctx, `UPDATE items SET vnum = $2 WHERE id = $1 AND character_id = $3`,
			upgraded.ID, upgraded.VNum, characterID)); err != nil {
			return err
		}
		for _, m := range spent {
			if err := keepLeft(ctx, tx, characterID, m); err != nil {
				return err
			}
		}
		return nil
	})
	return gold, err
}
