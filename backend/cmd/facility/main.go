// Command facility serves the HTTP API of the Facility context.
//
// Configuration, from the environment:
//
//	DATABASE_URL  the PostgreSQL URL of the runtime role of Facility (required)
//	HTTP_ADDR     the address to listen on (default ":8080")
//	LOG_LEVEL     debug, info, warn, or error (default "info")
//
// With the argument healthcheck, it asks the service that runs on this host
// for its health (GET /healthz on HTTP_ADDR) and exits with 0 when the service
// is healthy and 1 when it is not. The container image, which has no shell,
// runs it as its health check.
package main

import (
	"context"
	"io"
	"time"

	// The time zone database, so that the names of time zones resolve in a
	// container image that has none.
	_ "time/tzdata"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/config"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/logging"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres"
)

func main() {
	httpserver.Main("facility", run)
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	env := config.NewReader(getenv)
	databaseURL := env.Required("DATABASE_URL")
	addr := env.Default(httpserver.AddrVariable, httpserver.DefaultAddr)
	logLevel := env.Default("LOG_LEVEL", "info")
	if err := env.Err(); err != nil {
		return err
	}
	logger, err := logging.New(stdout, logLevel, "facility")
	if err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	handler, err := facility.NewHandler(facility.Deps{Pool: pool, Logger: logger, Now: time.Now})
	if err != nil {
		return err
	}
	return httpserver.Run(ctx, addr, handler, logger)
}
