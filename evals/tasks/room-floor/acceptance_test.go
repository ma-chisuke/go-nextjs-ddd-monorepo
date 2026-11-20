package http_test

// The acceptance test of the task room-floor (evals/). The grader copies this
// file into backend/internal/facility/internal/adapter/http/ and runs it
// there, over the in-memory store of the API tests. It speaks to the API
// alone and reads the answers as JSON, so it does not depend on how the floor
// is written in Go, and it sets up its own API, so it does not depend on the
// helpers of the tests next to it. acceptance_integration_test.go runs the
// same scenario over the real store.

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	facilityhttp "github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/http"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/postgres/postgrestest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/apitest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
)

// evalClientOver returns a client of the Facility API over the queries.
func evalClientOver(t *testing.T, queries postgres.Querier) *apitest.Client {
	t.Helper()
	rooms := app.NewRooms(queries, func() time.Time { return time.Date(2026, time.November, 18, 9, 0, 0, 0, time.UTC) })
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

func evalNewRoom(name string, floor any) map[string]any {
	room := map[string]any{
		"name":      name,
		"capacity":  8,
		"openFrom":  "09:00",
		"openUntil": "18:00",
		"timeZone":  "Asia/Tokyo",
	}
	if floor != nil {
		room["floor"] = floor
	}
	return room
}

// evalFloor returns the floor of a room as the JSON of the answer gives it:
// "null" for null, "absent" when the room has no property floor.
func evalFloor(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var room map[string]json.RawMessage
	if err := json.Unmarshal(raw, &room); err != nil {
		t.Fatalf("the room is not a JSON object: %v\n%s", err, raw)
	}
	floor, ok := room["floor"]
	if !ok {
		return "absent"
	}
	return string(floor)
}

// evalScenario registers rooms with and without a floor, and reads them back
// alone and in the list. prefix starts the name of every room, so that the
// scenario can share a database with other tests.
func evalScenario(t *testing.T, client *apitest.Client, prefix string) {
	t.Helper()
	rooms := []struct {
		name  string
		floor any
		want  string
	}{
		{"Aoba", 3, "3"},
		{"Basement", -5, "-5"},
		{"Ground", 0, "0"},
		{"Top", 200, "200"},
		{"Nowhere", nil, "null"},
	}
	ids := map[string]string{}
	for _, room := range rooms {
		res := client.Do(http.MethodPost, "/rooms", evalNewRoom(prefix+room.name, room.floor))
		if res.Status != http.StatusCreated {
			t.Fatalf("registering %s on the floor %v: got status %d, want 201\n%s", room.name, room.floor, res.Status, res.Body)
		}
		if got := evalFloor(t, res.Body); got != room.want {
			t.Errorf("the registration of %s: got the floor %s, want %s", room.name, got, room.want)
		}
		var id struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(res.Body, &id); err != nil {
			t.Fatal(err)
		}
		ids[room.name] = id.ID
	}

	// Each room alone.
	for _, room := range rooms {
		res := client.Do(http.MethodGet, "/rooms/"+ids[room.name], nil)
		if res.Status != http.StatusOK {
			t.Fatalf("getting %s: got status %d, want 200\n%s", room.name, res.Status, res.Body)
		}
		if got := evalFloor(t, res.Body); got != room.want {
			t.Errorf("the room %s: got the floor %s, want %s", room.name, got, room.want)
		}
	}

	// The rooms in the list.
	listed := map[string]string{}
	target := "/rooms?limit=100"
	for range 100 {
		res := client.Do(http.MethodGet, target, nil)
		if res.Status != http.StatusOK {
			t.Fatalf("the list: got status %d, want 200\n%s", res.Status, res.Body)
		}
		var page struct {
			Items      []json.RawMessage `json:"items"`
			NextCursor *string           `json:"nextCursor"`
		}
		if err := json.Unmarshal(res.Body, &page); err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			var room struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(item, &room); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(room.Name, prefix) {
				listed[strings.TrimPrefix(room.Name, prefix)] = evalFloor(t, item)
			}
		}
		if page.NextCursor == nil {
			break
		}
		target = "/rooms?limit=100&cursor=" + *page.NextCursor
	}
	for _, room := range rooms {
		if got := listed[room.name]; got != room.want {
			t.Errorf("the room %s in the list: got the floor %q, want %s", room.name, got, room.want)
		}
	}

	// A floor outside -5 to 200 is refused, as other invalid values are.
	for _, floor := range []any{201, -6} {
		res := client.Do(http.MethodPost, "/rooms", evalNewRoom(prefix+"Outside", floor))
		var p problem.Problem
		res.JSON(t, &p)
		if res.Status != http.StatusUnprocessableEntity || !strings.HasSuffix(p.Type, "/validation-error.md") {
			t.Errorf("registering a room on the floor %v: got status %d and type %q, want status 422 and the problem validation-error\n%s", floor, res.Status, p.Type, res.Body)
		}
	}
}

func TestEvalRoomFloor(t *testing.T) {
	t.Parallel()
	evalScenario(t, evalClientOver(t, &postgrestest.Fake{}), "")
}
