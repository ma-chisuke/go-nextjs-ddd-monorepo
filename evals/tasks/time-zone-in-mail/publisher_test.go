//go:build integration

package postgres_test

// The acceptance test of the task time-zone-in-mail (evals/) on the side of
// Reservation. The grader copies this file into
// backend/internal/reservation/internal/adapter/postgres/ and runs it against
// a database of its own. It books and reschedules through the API of
// Reservation, which asks a stand-in of the Facility API for the room, and
// reads the events that the relay would publish from the outbox: what
// Reservation publishes, whatever the code in between looks like. Its names
// start with eval, so that it meets nothing of the tests next to it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/apitest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event/eventtest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres/postgrestest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/facility"
	reservationhttp "github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/http"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app/apptest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// evalFacility answers GET /rooms/{roomId} as the Facility API does, for one
// room in Tokyo.
func evalFacility(t *testing.T, roomID uuid.UUID) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rooms/"+roomID.String() {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"type":"https://example.com/not-found.md","title":"Not found","status":404}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":%q,"name":"Eval","capacity":8,"openFrom":"09:00","openUntil":"18:00",`+
			`"timeZone":"Asia/Tokyo","createdAt":"2026-11-01T00:00:00Z"}`, roomID)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestEvalConfirmedEventsNameTheTimeZone(t *testing.T) {
	pool := postgrestest.Open(t, "reservation")
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	roomID := uuid.Must(uuid.NewV7())
	rooms, err := facility.NewDirectory(facility.Config{BaseURL: evalFacility(t, roomID)})
	if err != nil {
		t.Fatal(err)
	}
	ids := app.IDGeneratorFunc(func() (domain.ReservationID, error) {
		id, err := uuid.NewV7()
		return domain.ReservationID(id), err
	})
	// It is 09:00 in Tokyo on 3 June 2030; the reservation is for the day
	// after.
	now := time.Date(2030, time.June, 3, 9, 0, 0, 0, tokyo)
	reservations := app.New(postgres.NewStore(pool), rooms, apptest.NewClock(now), ids)
	handler, err := reservationhttp.NewHandler(reservations, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := reservationhttp.Document()
	if err != nil {
		t.Fatal(err)
	}
	client := apitest.New(t, doc, handler)

	start := time.Date(2030, time.June, 4, 10, 0, 0, 0, tokyo)
	res := client.Do(http.MethodPost, "/reservations", map[string]any{
		"roomId":         roomID,
		"startsAt":       start.Format(time.RFC3339),
		"endsAt":         start.Add(time.Hour).Format(time.RFC3339),
		"attendees":      3,
		"organizerEmail": "organizer@example.com",
	}, apitest.Header("Idempotency-Key", "eval-"+roomID.String()))
	if res.Status != http.StatusCreated {
		t.Fatalf("the booking: got status %d, want 201\n%s", res.Status, res.Body)
	}
	var booked struct {
		ID string `json:"id"`
	}
	res.JSON(t, &booked)
	later := time.Date(2030, time.June, 4, 14, 0, 0, 0, tokyo)
	res = client.Do(http.MethodPatch, "/reservations/"+booked.ID, map[string]any{
		"startsAt": later.Format(time.RFC3339),
		"endsAt":   later.Add(90 * time.Minute).Format(time.RFC3339),
	}, apitest.Header("If-Match", res.Header.Get("ETag")))
	if res.Status != http.StatusOK {
		t.Fatalf("the rescheduling: got status %d, want 200\n%s", res.Status, res.Body)
	}

	// The events of the reservation, as the relay would publish them; the
	// others stay in the outbox.
	var published []event.Envelope
	notMine := errors.New("an event of another reservation")
	outbox := postgres.NewOutbox(pool, time.Now, slog.New(slog.DiscardHandler))
	if _, err := outbox.Drain(context.Background(), 1000, func(_ context.Context, e event.Envelope) error {
		if e.Subject != booked.ID {
			return notMine
		}
		published = append(published, e)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(published) != 2 {
		t.Fatalf("the booking and the rescheduling published %d events, want 2", len(published))
	}
	for i, e := range published {
		if e.Type != "reservation.ReservationConfirmed.v1" {
			t.Errorf("event %d: got the type %s, want reservation.ReservationConfirmed.v1", i, e.Type)
			continue
		}
		eventtest.AssertMatchesContract(t, e)
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err != nil {
			t.Fatal(err)
		}
		if got := data["timeZone"]; got != "Asia/Tokyo" {
			t.Errorf("event %d: got the time zone %v, want Asia/Tokyo\n%s", i, got, e.Data)
		}
	}
}
