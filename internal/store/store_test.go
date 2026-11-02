package store

import (
	"context"
	"crypto/rand"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/config"
)

// freshSchema returns a URL whose connections see an empty schema of their own, so the
// test can migrate from nothing without touching the tables other tests use.
func freshSchema(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("needs PostgreSQL; run without -short")
	}
	base := config.TestDatabaseURL()
	if base == "" {
		t.Fatal("TEST_DATABASE_URL is not set: copy .env.example to .env and run `make db`, or pass -short")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	schema := "migrate_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

func TestServersStartingAtOnceMigrateAFreshDatabaseTogether(t *testing.T) {
	url := freshSchema(t)
	var g errgroup.Group
	for range 20 {
		g.Go(func() error {
			pool, err := Open(context.Background(), url)
			if err != nil {
				return err
			}
			pool.Close()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
}

// A kingdom change that waits on a character being created must see that character once
// it runs, and refuse.
func TestAKingdomChangeWaitingOnACharacterCreationIsRefused(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, freshSchema(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	accounts := Accounts{Pool: pool}
	id, err := accounts.CreateAccount(ctx, "admin", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.ChooseEmpire(ctx, id, account.EmpireShinsoo); err != nil {
		t.Fatal(err)
	}

	creation, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = creation.Rollback(ctx) }()
	var holder int32
	if err := creation.QueryRow(ctx, `SELECT pg_backend_pid() FROM accounts WHERE id = $1 FOR NO KEY UPDATE`, id).
		Scan(&holder); err != nil {
		t.Fatal(err)
	}
	type result struct {
		chosen bool
		err    error
	}
	change := make(chan result, 1)
	go func() {
		chosen, err := accounts.ChooseEmpire(ctx, id, account.EmpireJinno)
		change <- result{chosen, err}
	}()
	waitUntilBlockedBy(t, pool, holder)
	if _, err := creation.Exec(ctx, `INSERT INTO characters (account_id, slot, name, class, sex,
		strength, vitality, dexterity, intelligence, x, y) VALUES ($1, 0, 'Hendra', 'sura', 'female', 5, 3, 3, 5, 0, 0)`,
		id); err != nil {
		t.Fatal(err)
	}
	if err := creation.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case r := <-change:
		if r.err != nil || r.chosen {
			t.Fatalf("the kingdom changed under a new character: chosen %v, err %v", r.chosen, r.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the kingdom change never finished")
	}
}

// waitUntilBlockedBy returns once some session waits on a lock the session holder holds,
// which proves the other side reached the lock before holder let go of it.
func waitUntilBlockedBy(t *testing.T, pool *pgxpool.Pool, holder int32) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		var blocked bool
		err := pool.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY (pg_blocking_pids(pid)))`, holder).
			Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
	}
	t.Fatal("nothing ever waited on the lock")
}
