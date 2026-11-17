// Package reservation is the Reservation bounded context: booking a meeting
// room, rescheduling, and cancelling. See docs/contexts/reservation/.
//
// Reservation is the core subdomain, so it has a domain model with ports and
// adapters (TPL-0003, TPL-0004). This package wires the adapters, the use
// cases, and the HTTP handler together, and only cmd/reservation uses it.
// Other contexts reach Reservation through its HTTP API, never through this
// package or its database schema (TPL-0005).
package reservation

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

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

// newReservationID makes the id of a new reservation: a UUID version 7
// (TPL-0012).
func newReservationID() (domain.ReservationID, error) {
	id, err := uuid.NewV7()
	return domain.ReservationID(id), err
}
