// Package postgrestest connects the integration tests to the local database
// (`make db-up`). The tests carry the build tag `integration` and run in
// `make check-all`.
package postgrestest

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres"
)

// Open returns a pool that is connected as the runtime role of the bounded
// context, the role that the service of the context uses. The pool is closed
// when the test ends.
func Open(t *testing.T, boundedContext string) *pgxpool.Pool {
	t.Helper()
	role := boundedContext + "_runtime"
	pool, err := postgres.Open(t.Context(), URL(role))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// A fresh context: the context of the test is canceled before Cleanup runs.
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("postgrestest: cannot reach the local database as %s; run `make db-up` first: %v", role, err)
	}
	return pool
}

// URL returns the URL of the local database for the role. Locally the
// password of a role is its name (db/local.sql).
func URL(role string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		role, role,
		env("PGHOST", "127.0.0.1"), env("POSTGRES_PORT", "15432"), env("PGDATABASE", "app"))
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
