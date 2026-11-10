// Package server wires the parts together: the database, the shared login keys and both
// ports, and stops them together.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/auth"
	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/game"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/store"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// timeouts close a connection that has not logged in 30 seconds after connecting, and
// give up on a client that stops reading; a real client answers the server within a second.
var timeouts = network.Timeouts{Login: 30 * time.Second, Write: 10 * time.Second}

// eventCheck is how often the server looks for an event starting or ending: one is
// switched on or off at most this late, and one created on the command line is seen.
const eventCheck = 10 * time.Second

// saveEvery is how often every player in the world is saved: a crash loses at most this
// much of their play.
const saveEvery = time.Minute

// Run serves both ports until ctx ends or one of them fails.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	authLn, err := net.Listen("tcp", cfg.AuthListen.String())
	if err != nil {
		return err
	}
	gameLn, err := net.Listen("tcp", cfg.GameListen.String())
	if err != nil {
		_ = authLn.Close()
		return err
	}
	return Serve(ctx, Ports{Auth: authLn, Game: gameLn}, pool, world.Spawns(), cfg, log)
}

// Ports are the two listeners; tests pass listeners on free ports.
type Ports struct {
	Auth, Game net.Listener
}

// Serve runs both ports on listeners that are already open, and the world loop with the
// mobs of spawns in it.
func Serve(ctx context.Context, ports Ports, pool *pgxpool.Pool, spawns []world.Spawn, cfg config.Config,
	log *slog.Logger) error {
	items := store.Items{Pool: pool}
	if err := checkItemKinds(ctx, items); err != nil {
		return err
	}
	accounts := store.Accounts{Pool: pool}
	keys := account.NewKeys(time.Now)
	clock := handshake.NewClock()
	authSvc := auth.NewService(accounts, keys, clock)
	gameSvc := game.NewService(game.Deps{
		Accounts: accounts, Whispers: store.Whispers{Pool: pool}, Items: items, Taskbar: store.Taskbar{Pool: pool},
		Skills: store.Skills{Pool: pool}, Keys: keys, Clock: clock, Public: cfg.GamePublic, Spawns: spawns, Log: log,
	})

	log.Info("listening", "auth", ports.Auth.Addr().String(), "game", ports.Game.Addr().String(),
		"game_public", cfg.GamePublic.String())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return network.Serve(ctx, ports.Auth, authSvc.NewSession, timeouts, log.With("port", "auth"))
	})
	g.Go(func() error {
		return network.Serve(ctx, ports.Game, gameSvc.NewSession, timeouts, log.With("port", "game"))
	})
	g.Go(func() error { return gameSvc.Run(ctx) })
	g.Go(func() error { return gameSvc.WatchEvents(ctx, store.Events{Pool: pool}, eventCheck, log) })
	g.Go(func() error { return gameSvc.SavePlayers(ctx, saveEvery, log) })
	return g.Wait()
}

// checkItemKinds refuses to start while a character owns, or is to be given, an item of a
// kind the game no longer has: the server could neither show it nor say what it is, and
// removing a kind should not quietly take items away.
func checkItemKinds(ctx context.Context, items store.Items) error {
	unknown, err := items.UnknownKinds(ctx, item.VNums())
	if err != nil {
		return err
	}
	if len(unknown) > 0 {
		return fmt.Errorf("items of kinds the game does not have are owned or ordered: %v", unknown)
	}
	return nil
}
