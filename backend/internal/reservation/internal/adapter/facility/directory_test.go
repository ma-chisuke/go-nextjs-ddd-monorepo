package facility_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/facility"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

var roomID = domain.RoomID{0x01, 0x93, 0xa6, 0xf0, 0x7b, 0x5c, 0x7e, 0x2a, 0x9c, 0x3d, 0x1f, 0x2e, 0x3d, 0x4c, 0x5b, 0x6a}

// A room as Facility describes it.
const aoba = `{
	"id": "0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a",
	"name": "Aoba",
	"capacity": 8,
	"openFrom": "09:00",
	"openUntil": "18:00",
	"timeZone": "Asia/Tokyo",
	"createdAt": "2026-11-18T09:00:00Z"
}`

func facilityProblem(slug string, status string) string {
	return `{"type": "https://github.com/ma-chisuke/go-nextjs-ddd-monorepo/blob/main/docs/problems/` + slug + `.md",
		"title": "A problem", "status": ` + status + `}`
}

// answer is what the stand-in for Facility answers.
type answer struct {
	status      int
	contentType string
	body        string
}

// newFacility starts a stand-in for the Facility API that gives the canned
// answer. Every request that it receives, and the canned answer itself, are
// checked against contracts/http/facility.openapi.yaml, so that the stand-in
// cannot drift from the API it stands for.
func newFacility(t *testing.T, canned answer) string {
	t.Helper()
	router := facilityRouter(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, params, err := router.FindRoute(r)
		if err != nil {
			t.Errorf("the adapter sent %s %s, which is not an operation of the Facility API: %v", r.Method, r.URL, err)
			w.WriteHeader(http.StatusTeapot)
			return
		}
		request := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: params,
			Route:      route,
			Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		}
		if err := openapi3filter.ValidateRequest(r.Context(), request); err != nil {
			t.Errorf("the request of the adapter does not match the Facility API document: %v", err)
		}
		header := http.Header{"Content-Type": {canned.contentType}}
		if err := openapi3filter.ValidateResponse(r.Context(), &openapi3filter.ResponseValidationInput{
			RequestValidationInput: request,
			Status:                 canned.status,
			Header:                 header,
			Body:                   io.NopCloser(strings.NewReader(canned.body)),
			Options:                &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
		}); err != nil {
			t.Errorf("the canned answer does not match the Facility API document: %v", err)
		}
		w.Header().Set("Content-Type", canned.contentType)
		w.WriteHeader(canned.status)
		_, _ = io.WriteString(w, canned.body)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// facilityRouter reads the API document of Facility from the repository. It
// reads the file, not a Go package of Facility: Reservation never imports
// the code of another context (TPL-0005).
func facilityRouter(t *testing.T) routers.Router {
	t.Helper()
	httpserver.RegisterFormats()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Walk up to the directory that holds the document itself: other files
	// of the root, such as AGENTS.md, also exist in subdirectories.
	document := filepath.Join("contracts", "http", "facility.openapi.yaml")
	for {
		if _, err := os.Stat(filepath.Join(dir, document)); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no %s above the working directory, so the repository root is unknown", document)
		}
		dir = parent
	}
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(dir, document))
	if err != nil {
		t.Fatal(err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func newDirectory(t *testing.T, baseURL string) *facility.Directory {
	t.Helper()
	d, err := facility.NewDirectory(facility.Config{BaseURL: baseURL})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestRoom(t *testing.T) {
	t.Parallel()
	d := newDirectory(t, newFacility(t, answer{http.StatusOK, "application/json", aoba}))
	room, err := d.Room(t.Context(), roomID)
	if err != nil {
		t.Fatal(err)
	}
	if room.ID != roomID || room.Capacity != 8 || room.Hours.Opens.String() != "09:00" ||
		room.Hours.Closes.String() != "18:00" || room.TimeZone.String() != "Asia/Tokyo" {
		t.Errorf("got %+v, want room Aoba for 8 people, open from 09:00 to 18:00 in Asia/Tokyo", room)
	}
}

func TestRoomFailures(t *testing.T) {
	t.Parallel()
	withZone := func(zone string) string {
		return strings.Replace(aoba, "Asia/Tokyo", zone, 1)
	}
	tests := []struct {
		name   string
		answer answer
		want   error
	}{
		{"no room has the id", answer{http.StatusNotFound, "application/problem+json", facilityProblem("room-not-found", "404")}, app.ErrUnknownRoom},
		{"the URL is not that of the API", answer{http.StatusNotFound, "application/problem+json", facilityProblem("not-found", "404")}, app.ErrRoomsUnavailable},
		{"Facility fails", answer{http.StatusInternalServerError, "application/problem+json", facilityProblem("internal-error", "500")}, app.ErrRoomsUnavailable},
		{"Facility refuses the request", answer{http.StatusUnprocessableEntity, "application/problem+json", facilityProblem("validation-error", "422")}, app.ErrRoomsUnavailable},
		{"the time zone is not known here", answer{http.StatusOK, "application/json", withZone("Mars/Olympus")}, app.ErrRoomsUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := newDirectory(t, newFacility(t, tt.answer)).Room(t.Context(), roomID)
			if !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRoomWhenFacilityIsAway(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	_, err := newDirectory(t, url).Room(t.Context(), roomID)
	if !errors.Is(err, app.ErrRoomsUnavailable) {
		t.Errorf("got %v, want app.ErrRoomsUnavailable", err)
	}
}

func TestRoomWhenFacilityHangs(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	d, err := facility.NewDirectory(facility.Config{BaseURL: server.URL, Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = d.Room(context.Background(), roomID)
	if !errors.Is(err, app.ErrRoomsUnavailable) {
		t.Errorf("got %v, want app.ErrRoomsUnavailable", err)
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Errorf("the lookup waited %s, longer than its timeout", waited)
	}
}

func TestNewDirectoryNeedsAnAbsoluteURL(t *testing.T) {
	t.Parallel()
	for _, url := range []string{"", "facility:8080", "/rooms", "http://"} {
		if _, err := facility.NewDirectory(facility.Config{BaseURL: url}); err == nil {
			t.Errorf("the base URL %q was accepted", url)
		}
	}
}
