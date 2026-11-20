package http_test

// The acceptance test of the task rename-room (evals/). The grader copies
// this file into backend/internal/facility/internal/adapter/http/ and runs it
// there, over the in-memory store of the API tests. It speaks to the API
// alone, so it does not depend on how the rename is written, and it sets up
// its own API, so it does not depend on the helpers of the tests next to it.
// acceptance_integration_test.go runs the same scenario over the real store.

import (
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

var evalRegisteredAt = time.Date(2026, time.November, 18, 9, 0, 0, 0, time.UTC)

// evalClientOver returns a client of the Facility API over the queries.
func evalClientOver(t *testing.T, queries postgres.Querier) *apitest.Client {
	t.Helper()
	rooms := app.NewRooms(queries, func() time.Time { return evalRegisteredAt })
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

func evalRegister(t *testing.T, client *apitest.Client, name string) facilityhttp.Room {
	t.Helper()
	res := client.Do(http.MethodPost, "/rooms", map[string]any{
		"name":      name,
		"capacity":  8,
		"openFrom":  "09:00",
		"openUntil": "18:00",
		"timeZone":  "Asia/Tokyo",
	})
	if res.Status != http.StatusCreated {
		t.Fatalf("registering the room %q: got status %d, want 201\n%s", name, res.Status, res.Body)
	}
	var room facilityhttp.Room
	res.JSON(t, &room)
	return room
}

func evalRename(client *apitest.Client, id, name string) apitest.Response {
	return client.Do(http.MethodPatch, "/rooms/"+id, map[string]any{"name": name})
}

func evalWantProblem(t *testing.T, what string, res apitest.Response, status int, slug string) {
	t.Helper()
	var p problem.Problem
	res.JSON(t, &p)
	if res.Status != status || !strings.HasSuffix(p.Type, "/"+slug+".md") {
		t.Errorf("%s: got status %d and type %q, want status %d and the problem %s\n%s", what, res.Status, p.Type, status, slug, res.Body)
	}
}

// evalWantRoom checks that res is the room, renamed to name, with everything
// else as it was registered.
func evalWantRoom(t *testing.T, what string, res apitest.Response, registered facilityhttp.Room, name string) {
	t.Helper()
	if res.Status != http.StatusOK {
		t.Fatalf("%s: got status %d, want 200\n%s", what, res.Status, res.Body)
	}
	var got facilityhttp.Room
	res.JSON(t, &got)
	want := registered
	want.Name = name
	if got.Id != want.Id || got.Name != want.Name || got.Capacity != want.Capacity || got.OpenFrom != want.OpenFrom ||
		got.OpenUntil != want.OpenUntil || got.TimeZone != want.TimeZone || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("%s: got the room %+v, want %+v", what, got, want)
	}
}

// evalRenameScenario renames rooms through the API. prefix starts the name
// of every room, so that the scenario can share a database with other tests.
func evalRenameScenario(t *testing.T, client *apitest.Client, prefix string) {
	t.Helper()
	typo := evalRegister(t, client, prefix+"Aoab")
	birch := evalRegister(t, client, prefix+"Birch")

	// The rename answers with the room, and the room keeps its new name.
	evalWantRoom(t, "the rename", evalRename(client, typo.Id.String(), prefix+"Aoba"), typo, prefix+"Aoba")
	evalWantRoom(t, "the room after the rename", client.Do(http.MethodGet, "/rooms/"+typo.Id.String(), nil), typo, prefix+"Aoba")

	// The list shows the new name, and no longer the old one.
	var names []string
	target := "/rooms?limit=100"
	for range 100 {
		res := client.Do(http.MethodGet, target, nil)
		if res.Status != http.StatusOK {
			t.Fatalf("the list: got status %d, want 200\n%s", res.Status, res.Body)
		}
		var page facilityhttp.RoomPage
		res.JSON(t, &page)
		for _, room := range page.Items {
			if strings.HasPrefix(room.Name, prefix) {
				names = append(names, strings.TrimPrefix(room.Name, prefix))
			}
		}
		if page.NextCursor == nil {
			break
		}
		target = "/rooms?limit=100&cursor=" + *page.NextCursor
	}
	if got := strings.Join(names, ","); got != "Aoba,Birch" {
		t.Errorf("the list holds the rooms %s, want Aoba,Birch", got)
	}

	// The old name is free again.
	evalRegister(t, client, prefix+"Aoab")

	// A name that another room has is refused, and the room keeps its name.
	evalWantProblem(t, "a rename to the name of another room", evalRename(client, birch.Id.String(), prefix+"Aoba"), http.StatusConflict, "room-name-taken")
	evalWantRoom(t, "the room after a refused rename", client.Do(http.MethodGet, "/rooms/"+birch.Id.String(), nil), birch, prefix+"Birch")

	// The name that the room has already is accepted.
	evalWantRoom(t, "a rename to the same name", evalRename(client, birch.Id.String(), prefix+"Birch"), birch, prefix+"Birch")

	// A room that does not exist.
	evalWantProblem(t, "a rename of a room that does not exist", evalRename(client, "0193a6f0-7b5c-7e2a-9c3d-00000000e404", prefix+"Cedar"), http.StatusNotFound, "room-not-found")

	// The rules of a name, as at registration.
	evalWantProblem(t, "an empty name", evalRename(client, birch.Id.String(), ""), http.StatusUnprocessableEntity, "validation-error")
	evalWantProblem(t, "a name of 101 characters", evalRename(client, birch.Id.String(), strings.Repeat("n", 101)), http.StatusUnprocessableEntity, "validation-error")
}

func TestEvalRenameRoom(t *testing.T) {
	t.Parallel()
	evalRenameScenario(t, evalClientOver(t, &postgrestest.Fake{}), "")
}
