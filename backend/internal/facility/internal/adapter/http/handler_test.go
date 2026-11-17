package http_test

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	facilityhttp "github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/http"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/postgres/postgrestest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/apitest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
)

var registeredAt = time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)

// newClient returns a client of the Facility API over an empty in-memory
// store. Every response it receives is checked against the API document.
func newClient(t *testing.T) *apitest.Client {
	t.Helper()
	rooms := app.NewRooms(&postgrestest.Fake{}, func() time.Time { return registeredAt })
	handler, err := facilityhttp.NewHandler(rooms, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := facilityhttp.Document()
	if err != nil {
		t.Fatal(err)
	}
	return apitest.New(t, doc, handler)
}

func newRoom(name string) map[string]any {
	return map[string]any{
		"name":      name,
		"capacity":  8,
		"openFrom":  "09:00",
		"openUntil": "18:00",
		"timeZone":  "Asia/Tokyo",
	}
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

func TestRegisterAndGetRoom(t *testing.T) {
	t.Parallel()
	client := newClient(t)

	res := client.Do(http.MethodPost, "/rooms", newRoom("Aoba"))
	if res.Status != http.StatusCreated {
		t.Fatalf("got status %d, want 201\n%s", res.Status, res.Body)
	}
	var created facilityhttp.Room
	res.JSON(t, &created)
	if created.Name != "Aoba" || created.Capacity != 8 || created.OpenFrom != "09:00" || created.OpenUntil != "18:00" || created.TimeZone != "Asia/Tokyo" {
		t.Errorf("the registered room is not the one that was sent: %+v", created)
	}
	if !created.CreatedAt.Equal(registeredAt) {
		t.Errorf("got createdAt %s, want %s", created.CreatedAt, registeredAt)
	}
	location := res.Header.Get("Location")
	if location != "/rooms/"+created.Id.String() {
		t.Errorf("got Location %q, want the URL of the new room", location)
	}

	res = client.Do(http.MethodGet, location, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("got status %d, want 200\n%s", res.Status, res.Body)
	}
	var got facilityhttp.Room
	res.JSON(t, &got)
	if got != created {
		t.Errorf("got %+v, want the registered room %+v", got, created)
	}
}

func TestRegisterRoomRejections(t *testing.T) {
	t.Parallel()
	with := func(key string, value any) map[string]any {
		room := newRoom("Aoba")
		room[key] = value
		return room
	}
	without := func(key string) map[string]any {
		room := newRoom("Aoba")
		delete(room, key)
		return room
	}
	tests := []struct {
		name    string
		body    any
		slug    string
		status  int
		pointer string
	}{
		{"the JSON is broken", []byte(`{"name":`), "malformed-request", http.StatusBadRequest, ""},
		{"capacity is zero", with("capacity", 0), "validation-error", http.StatusUnprocessableEntity, "/capacity"},
		{"capacity is over the limit", with("capacity", 1001), "validation-error", http.StatusUnprocessableEntity, "/capacity"},
		{"name is empty", with("name", ""), "validation-error", http.StatusUnprocessableEntity, "/name"},
		{"name is missing", without("name"), "validation-error", http.StatusUnprocessableEntity, "/name"},
		{"openFrom is not a time of day", with("openFrom", "9am"), "validation-error", http.StatusUnprocessableEntity, "/openFrom"},
		// The error points at the object that holds the unknown property.
		{"a property is unknown", with("floor", 3), "validation-error", http.StatusUnprocessableEntity, ""},
		{"the room closes before it opens", with("openUntil", "08:00"), "invalid-opening-hours", http.StatusUnprocessableEntity, ""},
		{"the room opens and closes at the same time", with("openUntil", "09:00"), "invalid-opening-hours", http.StatusUnprocessableEntity, ""},
		{"the time zone does not exist", with("timeZone", "Mars/Olympus"), "unknown-time-zone", http.StatusUnprocessableEntity, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := newClient(t).Do(http.MethodPost, "/rooms", tt.body)
			p := wantProblem(t, res, tt.slug, tt.status)
			if tt.pointer == "" {
				return
			}
			for _, invalid := range p.Errors {
				if invalid.Pointer == tt.pointer {
					return
				}
			}
			t.Errorf("no error points at %q: %+v", tt.pointer, p.Errors)
		})
	}
}

func TestRegisterRoomWithAnotherMediaType(t *testing.T) {
	t.Parallel()
	res := newClient(t).Do(http.MethodPost, "/rooms", []byte("name=Aoba"), apitest.Header("Content-Type", "text/plain"))
	wantProblem(t, res, "unsupported-media-type", http.StatusUnsupportedMediaType)
}

func TestRegisterRoomWithATakenName(t *testing.T) {
	t.Parallel()
	client := newClient(t)
	client.Do(http.MethodPost, "/rooms", newRoom("Aoba"))
	res := client.Do(http.MethodPost, "/rooms", newRoom("Aoba"))
	wantProblem(t, res, "room-name-taken", http.StatusConflict)
}

func TestGetRoomRejections(t *testing.T) {
	t.Parallel()
	client := newClient(t)
	wantProblem(t, client.Do(http.MethodGet, "/rooms/0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a", nil), "room-not-found", http.StatusNotFound)
	wantProblem(t, client.Do(http.MethodGet, "/rooms/not-a-uuid", nil), "validation-error", http.StatusUnprocessableEntity)
}

func TestListRoomsInPages(t *testing.T) {
	t.Parallel()
	client := newClient(t)
	for _, name := range []string{"Cedar", "Aoba", "Birch"} {
		client.Do(http.MethodPost, "/rooms", newRoom(name))
	}

	var names []string
	target := "/rooms?limit=2"
	for pages := 0; ; pages++ {
		if pages > 2 {
			t.Fatal("the listing does not end")
		}
		res := client.Do(http.MethodGet, target, nil)
		if res.Status != http.StatusOK {
			t.Fatalf("got status %d, want 200\n%s", res.Status, res.Body)
		}
		var page facilityhttp.RoomPage
		res.JSON(t, &page)
		if len(page.Items) > 2 {
			t.Errorf("a page holds %d rooms, and the limit is 2", len(page.Items))
		}
		for _, room := range page.Items {
			names = append(names, room.Name)
		}
		if page.NextCursor == nil {
			break
		}
		target = "/rooms?limit=2&cursor=" + *page.NextCursor
	}
	if got, want := strings.Join(names, ","), "Aoba,Birch,Cedar"; got != want {
		t.Errorf("got the rooms %s, want %s, each once and in the order of their names", got, want)
	}
}

func TestListRoomsRejections(t *testing.T) {
	t.Parallel()
	client := newClient(t)
	wantProblem(t, client.Do(http.MethodGet, "/rooms?limit=0", nil), "validation-error", http.StatusUnprocessableEntity)
	wantProblem(t, client.Do(http.MethodGet, "/rooms?limit=101", nil), "validation-error", http.StatusUnprocessableEntity)
	wantProblem(t, client.Do(http.MethodGet, "/rooms?cursor=not-a-cursor", nil), "invalid-cursor", http.StatusUnprocessableEntity)
}

func TestRequestsOutsideTheDocument(t *testing.T) {
	t.Parallel()
	client := newClient(t)
	wantProblem(t, client.Do(http.MethodGet, "/meeting-rooms", nil), "not-found", http.StatusNotFound)
	wantProblem(t, client.Do(http.MethodDelete, "/rooms", nil), "method-not-allowed", http.StatusMethodNotAllowed)
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	rooms := app.NewRooms(&postgrestest.Fake{}, time.Now)
	handler, err := facilityhttp.NewHandler(rooms, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	res := apitest.Raw(handler, http.MethodGet, "/healthz")
	if res.Status != http.StatusNoContent {
		t.Errorf("got status %d, want 204", res.Status)
	}
}
