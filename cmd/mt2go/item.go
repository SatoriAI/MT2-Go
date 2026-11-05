package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/store"
)

func runItem(ctx context.Context, cfg config.Config, args []string) error {
	var do func(store.Items) error
	switch {
	case (len(args) == 3 || len(args) == 4) && args[0] == "give":
		count := "1"
		if len(args) == 4 {
			count = args[3]
		}
		do = func(items store.Items) error { return giveItem(ctx, items, args[1], args[2], count) }
	case len(args) == 3 && args[0] == "take":
		do = func(items store.Items) error { return takeItem(ctx, items, args[1], args[2]) }
	case len(args) == 2 && args[0] == "list":
		do = func(items store.Items) error { return listItems(ctx, items, args[1]) }
	default:
		return errors.New(usage)
	}
	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	return do(store.Items{Pool: pool})
}

func giveItem(ctx context.Context, items store.Items, character, vnumArg, countArg string) error {
	vnum, err := strconv.ParseUint(vnumArg, 10, 32)
	if err != nil {
		return fmt.Errorf("the item %q is not a number", vnumArg)
	}
	count, err := strconv.Atoi(countArg)
	if err != nil {
		return fmt.Errorf("the count %q is not a number", countArg)
	}
	o, err := item.GiveOrder(uint32(vnum), count)
	if err != nil {
		return err
	}
	id, err := items.Order(ctx, character, o)
	if err != nil {
		return err
	}
	fmt.Printf("order %d: %s gets %s, the next time they are selected\n", id, character, describeItem(o.VNum, o.Count))
	return nil
}

func takeItem(ctx context.Context, items store.Items, character, idArg string) error {
	itemID, err := strconv.ParseInt(idArg, 10, 64)
	if err != nil {
		return fmt.Errorf("the item id %q is not a number", idArg)
	}
	id, err := items.Order(ctx, character, item.Order{Kind: item.Take, ItemID: itemID})
	if err != nil {
		return err
	}
	fmt.Printf("order %d: %s loses item %d, the next time they are selected\n", id, character, itemID)
	return nil
}

func listItems(ctx context.Context, items store.Items, character string) error {
	inv, orders, err := items.Owned(ctx, character)
	if err != nil {
		return err
	}
	for _, it := range inv {
		fmt.Printf("item %d: %s in cell %d\n", it.ID, describeItem(it.VNum, it.Count), it.Cell)
	}
	for _, o := range orders {
		if o.Kind == item.Give {
			fmt.Printf("order %d: give %s\n", o.ID, describeItem(o.VNum, o.Count))
		} else {
			fmt.Printf("order %d: take item %d\n", o.ID, o.ItemID)
		}
	}
	return nil
}

func describeItem(vnum uint32, count uint8) string {
	p, _ := item.Lookup(vnum)
	if count == 1 {
		return fmt.Sprintf("%s (%d)", p.Name, vnum)
	}
	return fmt.Sprintf("%d × %s (%d)", count, p.Name, vnum)
}
