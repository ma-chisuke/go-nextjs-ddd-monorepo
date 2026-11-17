// Command reservation serves the HTTP API of the Reservation context.
//
// Configuration, from the environment:
//
//	DATABASE_URL      the PostgreSQL URL of the runtime role of Reservation (required)
//	FACILITY_API_URL  the base URL of the Facility API, such as http://facility:8080 (required)
//	HTTP_ADDR         the address to listen on (default ":8080")
//	LOG_LEVEL         debug, info, warn, or error (default "info")
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	// The time zone database, so that the time zones of rooms resolve in a
	// container image that has none.
	_ "time/tzdata"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/config"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/logging"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Getenv, os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "reservation:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	env := config.NewReader(getenv)
	databaseURL := env.Required("DATABASE_URL")
	facilityAPIURL := env.Required("FACILITY_API_URL")
	addr := env.Default("HTTP_ADDR", ":8080")
	logLevel := env.Default("LOG_LEVEL", "info")
	if err := env.Err(); err != nil {
		return err
	}
	logger, err := logging.New(stdout, logLevel, "reservation")
	if err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	handler, err := reservation.NewHandler(reservation.Deps{
		Pool:           pool,
		Logger:         logger,
		Now:            time.Now,
		FacilityAPIURL: facilityAPIURL,
	})
	if err != nil {
		return err
	}
	return httpserver.Run(ctx, addr, handler, logger)
}
