package store

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

// accountInShinsoo is a fresh database with one account, in Shinsoo, without characters.
func accountInShinsoo(t *testing.T) (Accounts, int64) {
	t.Helper()
	ctx := context.Background()
	pool, err := Open(ctx, freshSchema(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	accounts := Accounts{Pool: pool}
	id, err := accounts.CreateAccount(ctx, "admin", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.ChooseEmpire(ctx, id, account.EmpireShinsoo); err != nil {
		t.Fatal(err)
	}
	return accounts, id
}

// holdAccountLock begins a transaction that holds the account row's lock, as the store's
// own transactions do, and returns it with its session's process id.
func holdAccountLock(t *testing.T, pool *pgxpool.Pool, id int64) (pgx.Tx, int32) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	var holder int32
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid() FROM accounts WHERE id = $1 FOR NO KEY UPDATE`, id).
		Scan(&holder); err != nil {
		t.Fatal(err)
	}
	return tx, holder
}

func receive[T any](t *testing.T, c <-chan T) T {
	t.Helper()
	select {
	case v := <-c:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("the waiting call never finished")
		panic("unreachable")
	}
}

// A kingdom change that waits on a character being created must see that character once
// it runs, and refuse.
func TestAKingdomChangeWaitingOnACharacterCreationIsRefused(t *testing.T) {
	ctx := context.Background()
	accounts, id := accountInShinsoo(t)
	creation, holder := holdAccountLock(t, accounts.Pool, id)
	type result struct {
		chosen bool
		err    error
	}
	change := make(chan result, 1)
	go func() {
		chosen, err := accounts.ChooseEmpire(ctx, id, account.EmpireJinno)
		change <- result{chosen, err}
	}()
	waitUntilBlockedBy(t, accounts.Pool, holder)
	if _, err := creation.Exec(ctx, `INSERT INTO characters (account_id, slot, name, class, sex,
		strength, vitality, dexterity, intelligence, x, y) VALUES ($1, 0, 'Hendra', 'sura', 'female', 5, 3, 3, 5, 0, 0)`,
		id); err != nil {
		t.Fatal(err)
	}
	if err := creation.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if r := receive(t, change); r.err != nil || r.chosen {
		t.Fatalf("the kingdom changed under a new character: chosen %v, err %v", r.chosen, r.err)
	}
}

// The mirror case: a creation that waits on a kingdom change must see the new kingdom
// once it runs, and refuse, since its start position came from the old one.
func TestACharacterCreationWaitingOnAKingdomChangeIsRefused(t *testing.T) {
	ctx := context.Background()
	accounts, id := accountInShinsoo(t)
	change, holder := holdAccountLock(t, accounts.Pool, id)
	created := make(chan error, 1)
	go func() {
		_, err := accounts.CreateCharacter(ctx, id, account.EmpireShinsoo, account.Character{
			Name: "Hendra", Class: account.Sura, Sex: account.Female, Level: 1,
			Stats: account.Stats{Vitality: 3, Intelligence: 5, Strength: 5, Dexterity: 3},
		})
		created <- err
	}()
	waitUntilBlockedBy(t, accounts.Pool, holder)
	if _, err := change.Exec(ctx, `UPDATE accounts SET empire = 'jinno' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if err := change.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := receive(t, created); !errors.Is(err, account.ErrKingdomChanged) {
		t.Fatalf("expected the creation to be refused, got %v", err)
	}
}

// waitUntilBlockedBy returns once some session waits on a lock the session holder holds.
// In these tests only the call under test can, so it proves that call reached the lock
// before holder let go of it.
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
	t.Fatal("nothing ever waited on the account lock: the call under test does not take it")
}
