package store

import (
	"context"
	"crypto/rand"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

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
