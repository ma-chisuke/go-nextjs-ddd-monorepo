package http_test

// The acceptance test of the task booking-horizon (evals/). The grader copies
// this file into backend/internal/reservation/internal/adapter/http/ and runs
// it there. It speaks to the API alone, so it does not depend on how the rule
// is written, and it sets up its own API, so it does not depend on the
// helpers of the tests next to it.

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/apitest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
	reservationhttp "github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/http"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app/apptest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

var (
	evalZone   = time.FixedZone("Asia/Tokyo", 9*60*60)
	evalRoomID = uuid.MustParse("0193a6f0-7b5c-7e2a-9c3d-00000000e001")
	// evalNow is 10:00 in Tokyo on 18 November 2026. 2160 hours later it is
	// 10:00 on 16 February 2027, inside the opening hours of the room.
	evalNow     = time.Date(2026, time.November, 18, 10, 0, 0, 0, evalZone)
	evalHorizon = evalNow.Add(2160 * time.Hour)
)

func evalClient(t *testing.T) *apitest.Client {
	t.Helper()
	room := domain.Room{
		ID:       domain.RoomID(evalRoomID),
		Capacity: 8,
		Hours:    domain.OpeningHours{Opens: domain.NewTimeOfDay(9, 0), Closes: domain.NewTimeOfDay(18, 0)},
		TimeZone: evalZone,
	}
	reservations := app.New(&apptest.Store{}, apptest.NewRooms(room), apptest.NewClock(evalNow), &apptest.IDs{})
	handler, err := reservationhttp.NewHandler(reservations, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := reservationhttp.Document()
	if err != nil {
		t.Fatal(err)
	}
	return apitest.New(t, doc, handler)
}

func evalBooking(start time.Time) map[string]any {
	return map[string]any{
		"roomId":         evalRoomID,
		"startsAt":       start.Format(time.RFC3339),
		"endsAt":         start.Add(time.Hour).Format(time.RFC3339),
		"attendees":      2,
		"organizerEmail": "organizer@example.com",
	}
}

func evalWantTooFarAhead(t *testing.T, res apitest.Response) {
	t.Helper()
	var p problem.Problem
	res.JSON(t, &p)
	if res.Status != http.StatusUnprocessableEntity || !strings.HasSuffix(p.Type, "/period-too-far-ahead.md") {
		t.Errorf("got status %d and type %q, want status 422 and the problem period-too-far-ahead\n%s", res.Status, p.Type, res.Body)
	}
	if p.Title != "The period starts too far ahead" {
		t.Errorf("got the title %q, want %q", p.Title, "The period starts too far ahead")
	}
	if got := res.Header.Get("Content-Type"); got != problem.MediaType {
		t.Errorf("got the content type %q, want %q", got, problem.MediaType)
	}
}

func TestEvalBookingHorizon(t *testing.T) {
	t.Parallel()
	client := evalClient(t)

	// Exactly 2160 hours ahead is still accepted.
	res := client.Do(http.MethodPost, "/reservations", evalBooking(evalHorizon), apitest.Header("Idempotency-Key", "eval-at-the-horizon"))
	if res.Status != http.StatusCreated {
		t.Fatalf("a booking that starts exactly 2160 hours ahead: got status %d, want 201\n%s", res.Status, res.Body)
	}
	var booked reservationhttp.Reservation
	res.JSON(t, &booked)
	etag := res.Header.Get("ETag")

	// The next step of the grid is too far.
	later := evalHorizon.Add(15 * time.Minute)
	res = client.Do(http.MethodPost, "/reservations", evalBooking(later), apitest.Header("Idempotency-Key", "eval-past-the-horizon"))
	evalWantTooFarAhead(t, res)

	// A change that moves the reservation past the horizon is refused too.
	change := map[string]any{
		"startsAt": later.Format(time.RFC3339),
		"endsAt":   later.Add(time.Hour).Format(time.RFC3339),
	}
	res = client.Do(http.MethodPatch, "/reservations/"+booked.Id.String(), change, apitest.Header("If-Match", etag))
	evalWantTooFarAhead(t, res)

	// A change that keeps the reservation inside the horizon still works.
	earlier := evalHorizon.Add(-24 * time.Hour)
	change = map[string]any{
		"startsAt": earlier.Format(time.RFC3339),
		"endsAt":   earlier.Add(time.Hour).Format(time.RFC3339),
	}
	res = client.Do(http.MethodPatch, "/reservations/"+booked.Id.String(), change, apitest.Header("If-Match", etag))
	if res.Status != http.StatusOK {
		t.Errorf("a change to a period inside the horizon: got status %d, want 200\n%s", res.Status, res.Body)
	}
}
