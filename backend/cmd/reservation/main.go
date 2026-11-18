// Command reservation runs the Reservation context: its HTTP API, or, with
// the argument relay, the relay that publishes its integration events. The
// two are separate processes of one image (TPL-0014).
//
// Configuration of the API, from the environment:
//
//	DATABASE_URL      the PostgreSQL URL of the runtime role of Reservation (required)
//	FACILITY_API_URL  the base URL of the Facility API, such as http://facility:8080 (required)
//	HTTP_ADDR         the address to listen on (default ":8080")
//	LOG_LEVEL         debug, info, warn, or error (default "info")
//
// Configuration of the relay:
//
//	DATABASE_URL    the PostgreSQL URL of the runtime role of Reservation (required)
//	EVENT_BUS_NAME  the name of the EventBridge event bus to publish to (required)
//	AWS_REGION      the AWS region (required; ECS sets it)
//	HTTP_ADDR       the address of the health check (default ":8080")
//	LOG_LEVEL       debug, info, warn, or error (default "info")
//
// The local environment also points the relay at AWS Emulator, with the
// variables that package awsconfig describes.
//
// With the argument healthcheck, it asks the process that runs on this host
// for its health (GET /healthz on HTTP_ADDR) and exits with 0 when the process
// is healthy and 1 when it is not. The container image, which has no shell,
// runs it as its health check, for the API and for the relay alike.
package main

import (
	"context"
	"io"
	"time"

	// The time zone database, so that the time zones of rooms resolve in a
	// container image that has none.
	_ "time/tzdata"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsconfig"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsevents"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/config"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/logging"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation"
)

func main() {
	httpserver.Main("reservation", run, httpserver.Mode{Name: "relay", Run: runRelay})
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	env := config.NewReader(getenv)
	databaseURL := env.Required("DATABASE_URL")
	facilityAPIURL := env.Required("FACILITY_API_URL")
	addr := env.Default(httpserver.AddrVariable, httpserver.DefaultAddr)
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

// runRelay publishes the events that the API wrote to the outbox of
// Reservation to the event bus, until ctx is canceled.
func runRelay(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	env := config.NewReader(getenv)
	databaseURL := env.Required("DATABASE_URL")
	bus := env.Required("EVENT_BUS_NAME")
	addr := env.Default(httpserver.AddrVariable, httpserver.DefaultAddr)
	logLevel := env.Default("LOG_LEVEL", "info")
	if err := env.Err(); err != nil {
		return err
	}
	logger, err := logging.New(stdout, logLevel, "reservation-relay")
	if err != nil {
		return err
	}
	cfg, err := awsconfig.Load(ctx, getenv)
	if err != nil {
		return err
	}
	// Before the relay marks anything as published: a bus that does not
	// exist must stop the relay, not swallow the events.
	publisher := awsevents.NewPublisher(cfg, bus)
	if err := publisher.Ready(ctx); err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	relay := reservation.NewRelay(reservation.RelayDeps{Pool: pool, Logger: logger, Now: time.Now}, publisher)
	return httpserver.RunWorker(ctx, addr, logger, relay.Run)
}
