// Package facility is the Facility bounded context: meeting rooms and when
// they can be used. See docs/contexts/facility/.
//
// This package wires the context together, and only cmd/facility uses it.
// Other contexts reach Facility through its HTTP API, never through this
// package or its database schema (TPL-0005).
package facility

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	facilityhttp "github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/http"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/app"
)

// Deps is what the context needs from the process that runs it.
type Deps struct {
	// Pool connects to the database as the runtime role of Facility.
	Pool *pgxpool.Pool
	// Logger receives the access log and the errors of the context.
	Logger *slog.Logger
	// Now gives the current time.
	Now func() time.Time
}

// NewHandler returns the HTTP handler of the Facility API.
func NewHandler(deps Deps) (http.Handler, error) {
	rooms := app.NewRooms(postgres.New(deps.Pool), deps.Now)
	return facilityhttp.NewHandler(rooms, deps.Logger)
}
