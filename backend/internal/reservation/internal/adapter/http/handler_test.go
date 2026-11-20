package http_test

import (
	"log/slog"
	"net/http"
	"net/url"
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
	tokyo  = time.FixedZone("Asia/Tokyo", 9*60*60)
	roomID = uuid.MustParse("0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a")
	room   = domain.Room{
		ID:       domain.RoomID(roomID),
		Capacity: 8,
		Hours:    domain.OpeningHours{Opens: domain.NewTimeOfDay(9, 0), Closes: domain.NewTimeOfDay(18, 0)},
		TimeZone: tokyo,
	}
	otherRoomID = uuid.MustParse("0193a6f0-7b5c-7e2a-9c3d-000000000002")
	// now is 10:00 in Tokyo on 18 November 2026.
	now = time.Date(2026, time.November, 18, 10, 0, 0, 0, tokyo)
)

type env struct {
	client *apitest.Client
	rooms  *apptest.Rooms
	clock  *apptest.Clock
}

// newEnv returns a client of the Reservation API over an empty in-memory
// store, with room and another room that Facility knows. Every response it
// receives is checked against the API document.
func newEnv(t *testing.T) env {
	t.Helper()
	other := room
	other.ID = domain.RoomID(otherRoomID)
	e := env{rooms: apptest.NewRooms(room, other), clock: apptest.NewClock(now)}
	reservations := app.New(&apptest.Store{}, e.rooms, e.clock, &apptest.IDs{})
	handler, err := reservationhttp.NewHandler(reservations, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := reservationhttp.Document()
	if err != nil {
		t.Fatal(err)
	}
	e.client = apitest.New(t, doc, handler)
	return e
}

// newReservation is the body of a booking of room on 20 November 2026, from
// and until the given times of day in Tokyo.
func newReservation(from, until string) map[string]any {
	return map[string]any{
		"roomId":         roomID,
		"startsAt":       "2026-11-20T" + from + ":00+09:00",
		"endsAt":         "2026-11-20T" + until + ":00+09:00",
		"attendees":      4,
		"organizerEmail": "organizer@example.com",
	}
}

func key(k string) apitest.Option {
	return apitest.Header("Idempotency-Key", k)
}

func ifMatch(etag string) apitest.Option {
	return apitest.Header("If-Match", etag)
}

// book books room and returns the reservation and its ETag.
func (e env) book(t *testing.T, k, from, until string) (reservationhttp.Reservation, string) {
	t.Helper()
	res := e.client.Do(http.MethodPost, "/reservations", newReservation(from, until), key(k))
	if res.Status != http.StatusCreated {
		t.Fatalf("got status %d, want 201\n%s", res.Status, res.Body)
	}
	var r reservationhttp.Reservation
	res.JSON(t, &r)
	return r, res.Header.Get("ETag")
}

// wantProblem checks that res is the problem of the given kind.
func wantProblem(t *testing.T, res apitest.Response, slug string, status int) problem.Problem {
	t.Helper()
	var p problem.Problem
	res.JSON(t, &p)
	if res.Status != status || !strings.HasSuffix(p.Type, "/"+slug+".md") {
		t.Errorf("got status %d and type %q, want status %d and the problem %q\n%s", res.Status, p.Type, status, slug, res.Body)
	}
	if got := res.Header.Get("Content-Type"); got != problem.MediaType {
		t.Errorf("got the content type %q, want %q", got, problem.MediaType)
	}
	return p
}

func wantPointer(t *testing.T, p problem.Problem, pointer string) {
	t.Helper()
	for _, invalid := range p.Errors {
		if invalid.Pointer == pointer {
			return
		}
	}
	t.Errorf("no error points at %q: %+v", pointer, p.Errors)
}

func TestBookAndGetReservation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	res := e.client.Do(http.MethodPost, "/reservations", newReservation("10:00", "11:00"), key("k1"))
	if res.Status != http.StatusCreated {
		t.Fatalf("got status %d, want 201\n%s", res.Status, res.Body)
	}
	var created reservationhttp.Reservation
	res.JSON(t, &created)
	want := reservationhttp.Reservation{
		Id:             created.Id,
		RoomId:         roomID,
		StartsAt:       time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC),
		EndsAt:         time.Date(2026, 11, 20, 2, 0, 0, 0, time.UTC),
		Attendees:      4,
		OrganizerEmail: "organizer@example.com",
		Status:         reservationhttp.Confirmed,
	}
	if created != want {
		t.Errorf("got %+v, want %+v, with the instants in UTC", created, want)
	}
	location := res.Header.Get("Location")
	if location != "/reservations/"+created.Id.String() {
		t.Errorf("got Location %q, want the URL of the new reservation", location)
	}
	if got := res.Header.Get("ETag"); got != `"1"` {
		t.Errorf("got ETag %s, want \"1\"", got)
	}

	res = e.client.Do(http.MethodGet, location, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("got status %d, want 200\n%s", res.Status, res.Body)
	}
	var got reservationhttp.Reservation
	res.JSON(t, &got)
	if got != created {
		t.Errorf("got %+v, want the booked reservation %+v", got, created)
	}
	if etag := res.Header.Get("ETag"); etag != `"1"` {
		t.Errorf("got ETag %s, want \"1\"", etag)
	}
}

func TestBookRoomIdempotently(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first, _ := e.book(t, "k1", "10:00", "11:00")

	// The same request again answers like the first one, and books nothing.
	again, _ := e.book(t, "k1", "10:00", "11:00")
	if again != first {
		t.Errorf("got %+v, want the reservation of the first request %+v", again, first)
	}
	var page reservationhttp.ReservationPage
	e.client.Do(http.MethodGet, "/reservations", nil).JSON(t, &page)
	if len(page.Items) != 1 {
		t.Errorf("%d reservations exist, want 1", len(page.Items))
	}

	res := e.client.Do(http.MethodPost, "/reservations", newReservation("12:00", "13:00"), key("k1"))
	wantProblem(t, res, "idempotency-key-reused", http.StatusUnprocessableEntity)

	res = e.client.Do(http.MethodPost, "/reservations", newReservation("12:00", "13:00"))
	p := wantProblem(t, res, "validation-error", http.StatusUnprocessableEntity)
	wantPointer(t, p, "Idempotency-Key")
}

func TestBookRoomRejections(t *testing.T) {
	t.Parallel()
	with := func(body map[string]any, name string, value any) map[string]any {
		body[name] = value
		return body
	}
	tests := []struct {
		name    string
		body    any
		slug    string
		status  int
		pointer string
	}{
		{"the JSON is broken", []byte(`{"roomId":`), "malformed-request", http.StatusBadRequest, ""},
		{"nobody attends", with(newReservation("10:00", "11:00"), "attendees", 0), "validation-error", http.StatusUnprocessableEntity, "/attendees"},
		{"the email address has no @", with(newReservation("10:00", "11:00"), "organizerEmail", "organizer"), "validation-error", http.StatusUnprocessableEntity, "/organizerEmail"},
		// PostgreSQL stores no U+0000 in text, so the document refuses it
		// before the store fails with 500.
		{"the email address holds U+0000", with(newReservation("10:00", "11:00"), "organizerEmail", "organizer\x00@example.com"), "validation-error", http.StatusUnprocessableEntity, "/organizerEmail"},
		{"the start is not an instant", with(newReservation("10:00", "11:00"), "startsAt", "tomorrow"), "validation-error", http.StatusUnprocessableEntity, "/startsAt"},
		{"the room is not a UUID", with(newReservation("10:00", "11:00"), "roomId", "Aoba"), "validation-error", http.StatusUnprocessableEntity, "/roomId"},
		{"a property is unknown", with(newReservation("10:00", "11:00"), "floor", 3), "validation-error", http.StatusUnprocessableEntity, ""},
		{"the period ends before it starts", newReservation("11:00", "10:00"), "period-out-of-order", http.StatusUnprocessableEntity, ""},
		{"the period starts off the quarter hour", newReservation("10:05", "11:00"), "period-misaligned", http.StatusUnprocessableEntity, ""},
		{"the period is too long", newReservation("10:00", "14:15"), "period-too-long", http.StatusUnprocessableEntity, ""},
		{"the room opens later", newReservation("08:45", "10:00"), "outside-opening-hours", http.StatusUnprocessableEntity, ""},
		{"the attendees do not fit", with(newReservation("10:00", "11:00"), "attendees", 9), "over-capacity", http.StatusUnprocessableEntity, ""},
		{"the room is not known", with(newReservation("10:00", "11:00"), "roomId", uuid.MustParse("0193a6f0-7b5c-7e2a-9c3d-0000000000ff")), "unknown-room", http.StatusUnprocessableEntity, ""},
		{
			"the period is in the past",
			with(with(newReservation("", ""), "startsAt", "2026-11-18T09:45:00+09:00"), "endsAt", "2026-11-18T10:45:00+09:00"),
			"period-in-past", http.StatusUnprocessableEntity, "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := newEnv(t).client.Do(http.MethodPost, "/reservations", tt.body, key("k1"))
			p := wantProblem(t, res, tt.slug, tt.status)
			if tt.pointer != "" {
				wantPointer(t, p, tt.pointer)
			}
		})
	}
}

func TestBookRoomThatIsBooked(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.book(t, "k1", "10:00", "12:00")
	res := e.client.Do(http.MethodPost, "/reservations", newReservation("11:00", "13:00"), key("k2"))
	wantProblem(t, res, "room-already-booked", http.StatusConflict)
	// One may end when the next starts.
	e.book(t, "k3", "12:00", "13:00")
}

func TestBookRoomWhenFacilityDoesNotAnswer(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.rooms.Fail(app.ErrRoomsUnavailable)
	res := e.client.Do(http.MethodPost, "/reservations", newReservation("10:00", "11:00"), key("k1"))
	wantProblem(t, res, "facility-unavailable", http.StatusServiceUnavailable)
	if got := res.Header.Get("Retry-After"); got != "10" {
		t.Errorf("got Retry-After %q, want 10", got)
	}
}

func TestGetReservationRejections(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	wantProblem(t, e.client.Do(http.MethodGet, "/reservations/0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a", nil), "reservation-not-found", http.StatusNotFound)
	wantProblem(t, e.client.Do(http.MethodGet, "/reservations/not-a-uuid", nil), "validation-error", http.StatusUnprocessableEntity)
}

func TestRescheduleReservation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	r, etag := e.book(t, "k1", "10:00", "11:00")
	e.book(t, "k2", "14:00", "15:00")
	target := "/reservations/" + r.Id.String()
	change := map[string]any{"startsAt": "2026-11-20T12:00:00+09:00", "endsAt": "2026-11-20T13:30:00+09:00", "attendees": 6}

	res := e.client.Do(http.MethodPatch, target, change, ifMatch(etag))
	if res.Status != http.StatusOK {
		t.Fatalf("got status %d, want 200\n%s", res.Status, res.Body)
	}
	var moved reservationhttp.Reservation
	res.JSON(t, &moved)
	if !moved.StartsAt.Equal(time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)) || !moved.EndsAt.Equal(time.Date(2026, 11, 20, 4, 30, 0, 0, time.UTC)) || moved.Attendees != 6 {
		t.Errorf("the reservation did not move: %+v", moved)
	}
	newETag := res.Header.Get("ETag")
	if newETag != `"2"` {
		t.Errorf("got ETag %s, want \"2\"", newETag)
	}

	// Only the attendees change; the period stays.
	res = e.client.Do(http.MethodPatch, target, map[string]any{"attendees": 2}, ifMatch(newETag))
	var fewer reservationhttp.Reservation
	res.JSON(t, &fewer)
	if res.Status != http.StatusOK || fewer.Attendees != 2 || !fewer.StartsAt.Equal(moved.StartsAt) || res.Header.Get("ETag") != `"3"` {
		t.Errorf("got %d %+v with ETag %s, want 2 attendees at the same period, ETag \"3\"", res.Status, fewer, res.Header.Get("ETag"))
	}

	tests := []struct {
		name    string
		target  string
		body    any
		options []apitest.Option
		slug    string
		status  int
	}{
		{"no If-Match", target, change, nil, "precondition-required", http.StatusPreconditionRequired},
		{"If-Match is *", target, change, []apitest.Option{ifMatch("*")}, "precondition-required", http.StatusPreconditionRequired},
		{"an old version", target, change, []apitest.Option{ifMatch(etag)}, "precondition-failed", http.StatusPreconditionFailed},
		{"a weak ETag", target, change, []apitest.Option{ifMatch(`W/"3"`)}, "precondition-failed", http.StatusPreconditionFailed},
		{"onto another reservation", target, map[string]any{"endsAt": "2026-11-20T14:30:00+09:00"}, []apitest.Option{ifMatch(`"3"`)}, "room-already-booked", http.StatusConflict},
		{"too many attendees", target, map[string]any{"attendees": 9}, []apitest.Option{ifMatch(`"3"`)}, "over-capacity", http.StatusUnprocessableEntity},
		{"nothing to change", target, map[string]any{}, []apitest.Option{ifMatch(`"3"`)}, "validation-error", http.StatusUnprocessableEntity},
		{"no such reservation", "/reservations/0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a", change, []apitest.Option{ifMatch(`"1"`)}, "reservation-not-found", http.StatusNotFound},
	}
	for _, tt := range tests {
		res := e.client.Do(http.MethodPatch, tt.target, tt.body, tt.options...)
		t.Run(tt.name, func(t *testing.T) {
			wantProblem(t, res, tt.slug, tt.status)
		})
	}

	e.rooms.Fail(app.ErrRoomsUnavailable)
	res = e.client.Do(http.MethodPatch, target, map[string]any{"attendees": 3}, ifMatch(`"3"`))
	wantProblem(t, res, "facility-unavailable", http.StatusServiceUnavailable)
	if got := res.Header.Get("Retry-After"); got != "10" {
		t.Errorf("got Retry-After %q, want 10", got)
	}
}

func TestCancelReservation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	r, etag := e.book(t, "k1", "10:00", "11:00")
	target := "/reservations/" + r.Id.String() + "/cancellation"

	wantProblem(t, e.client.Do(http.MethodPost, target, nil), "precondition-required", http.StatusPreconditionRequired)
	wantProblem(t, e.client.Do(http.MethodPost, target, nil, ifMatch(`"2"`)), "precondition-failed", http.StatusPreconditionFailed)

	res := e.client.Do(http.MethodPost, target, nil, ifMatch(etag))
	if res.Status != http.StatusOK {
		t.Fatalf("got status %d, want 200\n%s", res.Status, res.Body)
	}
	var cancelled reservationhttp.Reservation
	res.JSON(t, &cancelled)
	if cancelled.Status != reservationhttp.Cancelled || cancelled.Id != r.Id || res.Header.Get("ETag") != `"2"` {
		t.Errorf("got %+v with ETag %s, want the reservation cancelled, with ETag \"2\"", cancelled, res.Header.Get("ETag"))
	}
	// The period is free again.
	e.book(t, "k2", "10:00", "11:00")

	// A cancelled reservation stays as it is.
	wantProblem(t, e.client.Do(http.MethodPost, target, nil, ifMatch(`"2"`)), "reservation-cancelled", http.StatusConflict)
	wantProblem(t, e.client.Do(http.MethodPatch, "/reservations/"+r.Id.String(), map[string]any{"attendees": 2}, ifMatch(`"2"`)),
		"reservation-cancelled", http.StatusConflict)

	wantProblem(t, e.client.Do(http.MethodPost, "/reservations/0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a/cancellation", nil, ifMatch(`"1"`)),
		"reservation-not-found", http.StatusNotFound)
}

func TestCancelReservationTooLate(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	r, etag := e.book(t, "k1", "10:00", "11:00")
	e.clock.Set(time.Date(2026, time.November, 20, 9, 31, 0, 0, tokyo))
	res := e.client.Do(http.MethodPost, "/reservations/"+r.Id.String()+"/cancellation", nil, ifMatch(etag))
	wantProblem(t, res, "cancellation-too-late", http.StatusUnprocessableEntity)
}

func TestListReservationsInPages(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	// Booked out of order: the list is in the order of the start.
	for _, from := range []string{"15:00", "09:00", "12:00", "10:00"} {
		e.book(t, "k"+from, from, from[:2]+":45")
	}
	other := newReservation("09:00", "10:00")
	other["roomId"] = otherRoomID
	if res := e.client.Do(http.MethodPost, "/reservations", other, key("other")); res.Status != http.StatusCreated {
		t.Fatalf("got status %d, want 201\n%s", res.Status, res.Body)
	}

	list := func(query url.Values) []string {
		t.Helper()
		var starts []string
		for pages := 0; ; pages++ {
			if pages > 5 {
				t.Fatal("the listing does not end")
			}
			res := e.client.Do(http.MethodGet, "/reservations?"+query.Encode(), nil)
			if res.Status != http.StatusOK {
				t.Fatalf("got status %d, want 200\n%s", res.Status, res.Body)
			}
			var page reservationhttp.ReservationPage
			res.JSON(t, &page)
			for _, r := range page.Items {
				starts = append(starts, r.StartsAt.In(tokyo).Format("15:04"))
			}
			if page.NextCursor == nil {
				return starts
			}
			query.Set("cursor", *page.NextCursor)
		}
	}
	tests := []struct {
		name  string
		query url.Values
		want  string
	}{
		{"all, two at a time", url.Values{"limit": {"2"}}, "09:00 09:00 10:00 12:00 15:00"},
		{"one room", url.Values{"roomId": {roomID.String()}, "limit": {"3"}}, "09:00 10:00 12:00 15:00"},
		// The window is from 10:00 to 15:00 in Tokyo. The reservations from
		// 9:00 end before it or when it starts, and the one from 15:00
		// starts when it ends.
		{
			"a window",
			url.Values{"from": {"2026-11-20T01:00:00Z"}, "to": {"2026-11-20T06:00:00Z"}, "limit": {"1"}},
			"10:00 12:00",
		},
	}
	for _, tt := range tests {
		if got := strings.Join(list(tt.query), " "); got != tt.want {
			t.Errorf("%s: got the starts %s, want %s, each once and in order", tt.name, got, tt.want)
		}
	}

	wantProblem(t, e.client.Do(http.MethodGet, "/reservations?cursor=not-a-cursor", nil), "invalid-cursor", http.StatusUnprocessableEntity)
	wantProblem(t, e.client.Do(http.MethodGet, "/reservations?limit=0", nil), "validation-error", http.StatusUnprocessableEntity)
	wantProblem(t, e.client.Do(http.MethodGet, "/reservations?limit=101", nil), "validation-error", http.StatusUnprocessableEntity)
	wantProblem(t, e.client.Do(http.MethodGet, "/reservations?roomId=Aoba", nil), "validation-error", http.StatusUnprocessableEntity)
	p := wantProblem(t, e.client.Do(http.MethodGet, "/reservations?from=2026-11-20T06:00:00Z&to=2026-11-20T06:00:00Z", nil),
		"validation-error", http.StatusUnprocessableEntity)
	wantPointer(t, p, "to")
}

func TestBodiesOfAnotherMediaType(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	r, etag := e.book(t, "k1", "10:00", "11:00")
	plain := apitest.Header("Content-Type", "text/plain")
	wantProblem(t, e.client.Do(http.MethodPost, "/reservations", []byte("10:00"), key("k2"), plain),
		"unsupported-media-type", http.StatusUnsupportedMediaType)
	wantProblem(t, e.client.Do(http.MethodPatch, "/reservations/"+r.Id.String(), []byte("10:00"), ifMatch(etag), plain),
		"unsupported-media-type", http.StatusUnsupportedMediaType)
}

func TestRequestsOutsideTheDocument(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	wantProblem(t, e.client.Do(http.MethodGet, "/bookings", nil), "not-found", http.StatusNotFound)
	tests := []struct{ method, path, allow string }{
		{http.MethodDelete, "/reservations", "GET, POST"},
		{http.MethodPut, "/reservations/0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a", "GET, PATCH"},
		{http.MethodGet, "/reservations/0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a/cancellation", "POST"},
	}
	for _, tt := range tests {
		res := e.client.Do(tt.method, tt.path, nil)
		wantProblem(t, res, "method-not-allowed", http.StatusMethodNotAllowed)
		if got := res.Header.Get("Allow"); got != tt.allow {
			t.Errorf("%s %s: got Allow %q, want %q", tt.method, tt.path, got, tt.allow)
		}
	}
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	reservations := app.New(&apptest.Store{}, apptest.NewRooms(), apptest.NewClock(now), &apptest.IDs{})
	handler, err := reservationhttp.NewHandler(reservations, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	res := apitest.Raw(handler, http.MethodGet, "/healthz")
	if res.Status != http.StatusNoContent {
		t.Errorf("got status %d, want 204", res.Status)
	}
}
