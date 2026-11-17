// Package facility asks the Facility context about rooms, over the client
// that oapi-codegen generates from contracts/http/facility.openapi.yaml
// (client.gen.go). It is the anticorruption layer of Reservation (TPL-0005):
// it translates the answers of Facility into domain.Room, and no type of
// Facility leaves this package.
package facility

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// DefaultTimeout bounds a request to Facility when Config names no timeout.
// A booking waits for the answer, so a Facility that hangs must not hold it
// for long.
const DefaultTimeout = 3 * time.Second

// roomNotFound is the problem with which Facility says that no room has the
// id. A 404 with any other problem, such as the one for a URL that does not
// exist, means that Facility cannot be asked as configured. The URI of a
// problem never changes (TPL-0008).
var roomNotFound = problem.Type{Slug: "room-not-found"}.URI()

// Config says how to reach the Facility API.
type Config struct {
	// BaseURL is the absolute URL of the Facility API, such as
	// http://facility:8080.
	BaseURL string
	// Timeout bounds each request. Zero means DefaultTimeout.
	Timeout time.Duration
	// HTTPClient sends the requests. Nil means a client of its own.
	HTTPClient *http.Client
}

// Directory implements app.RoomDirectory over the Facility API.
type Directory struct {
	client  ClientWithResponsesInterface
	timeout time.Duration
}

var _ app.RoomDirectory = (*Directory)(nil)

// NewDirectory returns the directory of the Facility API that cfg names.
func NewDirectory(cfg Config) (*Directory, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("facility: %q is not an absolute URL of the Facility API", cfg.BaseURL)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	client, err := NewClientWithResponses(cfg.BaseURL, WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("facility: %w", err)
	}
	return &Directory{client: client, timeout: timeout}, nil
}

// Room asks Facility for the room with the id. It returns app.ErrUnknownRoom
// when Facility says that no room has the id, and app.ErrRoomsUnavailable when
// Facility does not answer in time, fails, or gives an answer that cannot be
// used.
func (d *Directory) Room(ctx context.Context, id domain.RoomID) (domain.Room, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	res, err := d.client.GetRoomWithResponse(ctx, uuid.UUID(id))
	if err != nil {
		return domain.Room{}, fmt.Errorf("%w: get room %s: %w", app.ErrRoomsUnavailable, id, err)
	}
	switch {
	case res.StatusCode() == http.StatusOK && res.JSON200 != nil:
		room, err := translate(id, *res.JSON200)
		if err != nil {
			return domain.Room{}, fmt.Errorf("%w: get room %s: %w", app.ErrRoomsUnavailable, id, err)
		}
		return room, nil
	case res.StatusCode() == http.StatusNotFound && res.ApplicationproblemJSON404 != nil &&
		res.ApplicationproblemJSON404.Type == roomNotFound:
		return domain.Room{}, app.ErrUnknownRoom
	}
	return domain.Room{}, fmt.Errorf("%w: get room %s: Facility answered %s", app.ErrRoomsUnavailable, id, res.Status())
}

// translate turns a room of Facility into a room of Reservation.
func translate(id domain.RoomID, room Room) (domain.Room, error) {
	opens, err := timeOfDay(room.OpenFrom)
	if err != nil {
		return domain.Room{}, fmt.Errorf("openFrom: %w", err)
	}
	closes, err := timeOfDay(room.OpenUntil)
	if err != nil {
		return domain.Room{}, fmt.Errorf("openUntil: %w", err)
	}
	location, err := time.LoadLocation(room.TimeZone)
	if err != nil {
		return domain.Room{}, fmt.Errorf("timeZone: %w", err)
	}
	if room.Capacity < 1 {
		return domain.Room{}, errors.New("the room holds nobody")
	}
	return domain.Room{
		ID:       id,
		Capacity: int(room.Capacity),
		Hours:    domain.OpeningHours{Opens: opens, Closes: closes},
		TimeZone: location,
	}, nil
}

// timeOfDay reads a time of day that Facility writes as HH:MM.
func timeOfDay(s string) (domain.TimeOfDay, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a time of day in the form HH:MM", s)
	}
	return domain.NewTimeOfDay(t.Hour(), t.Minute()), nil
}
