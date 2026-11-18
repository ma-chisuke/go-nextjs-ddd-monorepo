// Package reservation is the Reservation bounded context: booking a meeting
// room, rescheduling, and cancelling. See docs/contexts/reservation/.
//
// Reservation is the core subdomain, so it has a domain model with ports and
// adapters (TPL-0003, TPL-0004). This package wires the adapters, the use
// cases, the HTTP handler, and the relay of the outbox together, and only
// cmd/reservation uses it. Other contexts reach Reservation through its HTTP
// API and its integration events, never through this package or its
// database schema (TPL-0005).
package reservation

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/outbox"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/facility"
	reservationhttp "github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/http"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// Deps is what the context needs from the process that runs it.
type Deps struct {
	// Pool connects to the database as the runtime role of Reservation.
	Pool *pgxpool.Pool
	// Logger receives the access log and the errors of the context.
	Logger *slog.Logger
	// Now gives the current time.
	Now func() time.Time
	// FacilityAPIURL is the base URL of the Facility API, which tells the
	// capacity and the opening hours of each room.
	FacilityAPIURL string
}

// NewHandler returns the HTTP handler of the Reservation API.
func NewHandler(deps Deps) (http.Handler, error) {
	rooms, err := facility.NewDirectory(facility.Config{BaseURL: deps.FacilityAPIURL})
	if err != nil {
		return nil, err
	}
	reservations := app.New(
		postgres.NewStore(deps.Pool),
		rooms,
		app.ClockFunc(deps.Now),
		app.IDGeneratorFunc(newReservationID),
	)
	return reservationhttp.NewHandler(reservations, deps.Logger)
}

// RelayDeps is what the relay of the outbox of Reservation needs from the
// process that runs it.
type RelayDeps struct {
	// Pool connects to the database as the runtime role of Reservation.
	Pool *pgxpool.Pool
	// Logger receives each event that the relay publishes, and its errors.
	Logger *slog.Logger
	// Now gives the current time.
	Now func() time.Time
	// Config sets the pace of the relay. Its zero value takes the defaults
	// of package outbox.
	Config outbox.Config
}

// NewRelay returns the relay that publishes the integration events of
// Reservation from its outbox to publisher, at least once each (TPL-0013).
// The process runs it with Run.
func NewRelay(deps RelayDeps, publisher outbox.Publisher) *outbox.Relay {
	source := postgres.NewOutbox(deps.Pool, deps.Now, deps.Logger)
	return outbox.NewRelay(source, publisher, deps.Logger, deps.Now, deps.Config)
}

// newReservationID makes the id of a new reservation: a UUID version 7
// (TPL-0012).
func newReservationID() (domain.ReservationID, error) {
	id, err := uuid.NewV7()
	return domain.ReservationID(id), err
}
