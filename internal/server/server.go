// Package server wires the parts together: the database, the shared login keys and both
// ports, and stops them together.
package server

import (
	"context"
	"log/slog"
	"net"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/auth"
	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/game"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/store"
)

// timeouts close a connection that has not logged in 30 seconds after connecting, and
// give up on a client that stops reading; a real client answers the server within a second.
var timeouts = network.Timeouts{Login: 30 * time.Second, Write: 10 * time.Second}

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
	return Serve(ctx, Ports{Auth: authLn, Game: gameLn}, store.Accounts{Pool: pool}, cfg, log)
}

// Ports are the two listeners; tests pass listeners on free ports.
type Ports struct {
	Auth, Game net.Listener
}

// Serve runs both ports on listeners that are already open.
func Serve(ctx context.Context, ports Ports, accounts store.Accounts, cfg config.Config, log *slog.Logger) error {
	keys := account.NewKeys(time.Now)
	clock := handshake.NewClock()
	authSvc, err := auth.NewService(accounts, keys, clock)
	if err != nil {
		return err
	}
	gameSvc := game.NewService(accounts, keys, clock, cfg.GamePublic)

	log.Info("listening", "auth", ports.Auth.Addr().String(), "game", ports.Game.Addr().String(),
		"game_public", cfg.GamePublic.String())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return network.Serve(ctx, ports.Auth, authSvc.NewSession, timeouts, log.With("port", "auth"))
	})
	g.Go(func() error {
		return network.Serve(ctx, ports.Game, gameSvc.NewSession, timeouts, log.With("port", "game"))
	})
	return g.Wait()
}
