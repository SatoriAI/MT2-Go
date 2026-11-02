// Command mt2go runs the server, and creates accounts while there is no website to
// register on.
//
//	mt2go serve
//	mt2go account create <name>   (password read from standard input)
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/crypto/bcrypt"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/server"
	"github.com/SatoriAI/MT2-Go/internal/store"
)

const usage = "usage: mt2go serve | mt2go account create <name>"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(ctx, log, os.Args[1:]); err != nil {
		log.Error("mt2go failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch {
	case len(args) == 1 && args[0] == "serve":
		return server.Run(ctx, cfg, log)
	case len(args) == 3 && args[0] == "account" && args[1] == "create":
		return createAccount(ctx, cfg, args[2])
	}
	return errors.New(usage)
}

func createAccount(ctx context.Context, cfg config.Config, name string) error {
	password, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading the password from standard input: %w", err)
	}
	password = strings.TrimRight(password, "\r\n")
	if name == "" || len(name) > account.MaxNameLength {
		return fmt.Errorf("the name must be 1 to %d bytes", account.MaxNameLength)
	}
	if password == "" || len(password) > account.MaxPasswordLength {
		return fmt.Errorf("the password must be 1 to %d characters: the client sends no more", account.MaxPasswordLength)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	id, err := store.Accounts{Pool: pool}.CreateAccount(ctx, name, hash)
	if err != nil {
		return err
	}
	fmt.Printf("created account %q (id %d)\n", name, id)
	return nil
}
