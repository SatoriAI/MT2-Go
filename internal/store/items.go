package store

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
)

// Items keeps what characters own, and the orders that change it.
type Items struct {
	Pool *pgxpool.Pool
}

// ErrNotOwned means the character owns no item with that id.
var ErrNotOwned = errors.New("store: the character owns no such item")

// characterID finds a character by name, in any case, or returns account.ErrNotFound.
func characterID(ctx context.Context, tx pgx.Tx, name string) (uint32, error) {
	var id uint32
	err := tx.QueryRow(ctx, `SELECT id FROM characters WHERE lower(name) = lower($1)`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, account.ErrNotFound
	}
	return id, err
}

// Order leaves an order for the character named, and returns its id. A take names an
// item the character owns now, or is refused with ErrNotOwned.
func (s Items) Order(ctx context.Context, character string, o item.Order) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		cid, err := characterID(ctx, tx, character)
		if err != nil {
			return err
		}
		if o.Kind == item.Take {
			var owned bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM items WHERE id = $1 AND character_id = $2)`,
				o.ItemID, cid).Scan(&owned); err != nil {
				return err
			}
			if !owned {
				return ErrNotOwned
			}
		}
		return tx.QueryRow(ctx, `INSERT INTO item_orders (character_id, kind, vnum, count, item_id)
			VALUES ($1, $2, nullif($3, 0), nullif($4, 0), nullif($5, 0)) RETURNING id`,
			cid, o.Kind, o.VNum, o.Count, o.ItemID).Scan(&id)
	})
	return id, err
}

// Owned returns what the character named owns and the orders waiting for it.
func (s Items) Owned(ctx context.Context, character string) (item.Inventory, []item.Order, error) {
	var inv item.Inventory
	var orders []item.Order
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		cid, err := characterID(ctx, tx, character)
		if err != nil {
			return err
		}
		if inv, err = inventory(ctx, tx, cid); err != nil {
			return err
		}
		orders, err = pending(ctx, tx, cid)
		return err
	})
	return inv, orders, err
}

// ApplyOrders carries out the character's orders, oldest first, in one transaction, and
// returns what the character then owns and the orders carried out. A give goes into the
// first cell its item fits; one that fits nowhere waits for room. A take whose item is
// gone is done with nothing to take.
func (s Items) ApplyOrders(ctx context.Context, characterID uint32) (item.Inventory, []item.Order, error) {
	var inv item.Inventory
	var done []item.Order
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		orders, err := pending(ctx, tx, characterID)
		if err != nil {
			return err
		}
		if inv, err = inventory(ctx, tx, characterID); err != nil {
			return err
		}
		for _, o := range orders {
			switch o.Kind {
			case item.Give:
				p, _ := item.Lookup(o.VNum)
				cell, ok := inv.Free(p)
				if !ok {
					continue
				}
				it := item.Item{VNum: o.VNum, Count: o.Count, Cell: cell}
				if err := tx.QueryRow(ctx, `INSERT INTO items (character_id, vnum, count, cell)
					VALUES ($1, $2, $3, $4) RETURNING id`, characterID, it.VNum, it.Count, it.Cell).
					Scan(&it.ID); err != nil {
					return err
				}
				inv = append(inv, it)
			case item.Take:
				if _, err := tx.Exec(ctx, `DELETE FROM items WHERE id = $1 AND character_id = $2`,
					o.ItemID, characterID); err != nil {
					return err
				}
				inv = slices.DeleteFunc(inv, func(it item.Item) bool { return it.ID == o.ItemID })
			}
			if _, err := tx.Exec(ctx, `DELETE FROM item_orders WHERE id = $1`, o.ID); err != nil {
				return err
			}
			done = append(done, o)
		}
		return nil
	})
	return inv, done, err
}

// Move puts the item's top at cell.
func (s Items) Move(ctx context.Context, id int64, cell uint16) error {
	_, err := s.Pool.Exec(ctx, `UPDATE items SET cell = $2 WHERE id = $1`, id, cell)
	return err
}

// UnknownKinds returns the kinds owned or ordered that are not among known, in order.
func (s Items) UnknownKinds(ctx context.Context, known []uint32) ([]uint32, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT vnum FROM items WHERE vnum <> ALL ($1)
		UNION SELECT vnum FROM item_orders WHERE vnum <> ALL ($1)
		ORDER BY vnum`, known)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uint32])
}

func inventory(ctx context.Context, tx pgx.Tx, characterID uint32) (item.Inventory, error) {
	rows, err := tx.Query(ctx, `SELECT id, vnum, count, cell FROM items WHERE character_id = $1 ORDER BY cell`,
		characterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (item.Item, error) {
		var it item.Item
		err := row.Scan(&it.ID, &it.VNum, &it.Count, &it.Cell)
		return it, err
	})
}

// pending returns the character's orders, oldest first.
func pending(ctx context.Context, tx pgx.Tx, characterID uint32) ([]item.Order, error) {
	rows, err := tx.Query(ctx, `SELECT id, kind, coalesce(vnum, 0), coalesce(count, 0), coalesce(item_id, 0)
		FROM item_orders WHERE character_id = $1 ORDER BY id`, characterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (item.Order, error) {
		var o item.Order
		err := row.Scan(&o.ID, &o.Kind, &o.VNum, &o.Count, &o.ItemID)
		return o, err
	})
}
