// Package postgres opens the connection pool of a service.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool settings. They let an idle database pause (TPL-0016): the pool keeps no
// connection open when nothing uses it, and gives a paused database time to
// resume when the first query arrives.
const (
	minConns        = 0
	maxConnIdleTime = 3 * time.Minute
	maxConnLifetime = 30 * time.Minute
	connectTimeout  = 30 * time.Second
)

// Open returns a pool for the database at url. It opens no connection yet:
// the first query does, so starting a service does not wake the database.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("postgres: the database URL is not valid: %w", err)
	}
	cfg.MinConns = minConns
	cfg.MaxConnIdleTime = maxConnIdleTime
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.ConnConfig.ConnectTimeout = connectTimeout
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	return pool, nil
}
