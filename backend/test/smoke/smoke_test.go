//go:build smoke

// Package smoke runs one scenario across the services of the local system,
// over real HTTP: Facility and Reservation in Docker Compose, with PostgreSQL
// behind them, and then the events of the scenario on their way to the
// organizer: from the outbox of Reservation through its relay, the event bus
// and the queue in AWS Emulator, and the worker of Notification, to Mailpit.
// `make smoke` starts the system and runs this test, which fails when the
// system is not up.
//
// The test knows the services only by their API documents in contracts/http/,
// which check every response, and by the mail that arrives; it imports no
// bounded context (TPL-0005).
package smoke

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/apitest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
)

// contracts is the directory of the API documents, from this package.
const contracts = "../../../contracts/http"

type reservation struct {
	ID       string `json:"id"`
	StartsAt string `json:"startsAt"`
	Status   string `json:"status"`
}

func TestBookARoomAcrossTheServices(t *testing.T) {
	facility := connect(t, "facility", "FACILITY_URL", "http://127.0.0.1:18081")
	reservations := connect(t, "reservation", "RESERVATION_URL", "http://127.0.0.1:18082")

	// A name that no other run uses, so the scenario never meets the rows of
	// an earlier run and leaves nothing that a later run could meet.
	run := uuid.NewString()
	// An organizer of this run alone, so that the mail of the scenario can be
	// told from the mail of every other run.
	organizer := "smoke-" + run + "@example.com"

	res := facility.Do(http.MethodPost, "/rooms", map[string]any{
		"name":      "Smoke test " + run,
		"capacity":  6,
		"openFrom":  "08:00",
		"openUntil": "20:00",
		"timeZone":  "UTC",
	})
	status(t, "register a room in Facility", res, http.StatusCreated)
	var room struct {
		ID string `json:"id"`
	}
	res.JSON(t, &room)

	// Tomorrow, so the period is in the future and far from the deadline of a
	// cancellation, on quarter hours, and inside the opening hours.
	y, m, d := time.Now().UTC().AddDate(0, 0, 1).Date()
	instant := func(hour int) time.Time { return time.Date(y, m, d, hour, 0, 0, 0, time.UTC) }
	at := func(hour int) string { return instant(hour).Format(time.RFC3339) }
	booking := map[string]any{
		"roomId":         room.ID,
		"startsAt":       at(10),
		"endsAt":         at(11),
		"attendees":      4,
		"organizerEmail": organizer,
	}
	key := apitest.Header("Idempotency-Key", "smoke-"+run)

	res = reservations.Do(http.MethodPost, "/reservations", booking, key)
	status(t, "book the room in Reservation, which asks Facility for the room", res, http.StatusCreated)
	var booked reservation
	res.JSON(t, &booked)
	etag := res.Header.Get("ETag")

	res = reservations.Do(http.MethodPost, "/reservations", booking, key)
	status(t, "repeat the booking with the same Idempotency-Key", res, http.StatusCreated)
	var repeated reservation
	res.JSON(t, &repeated)
	if repeated.ID != booked.ID {
		t.Fatalf("the repeated booking made the reservation %s, want the first one, %s", repeated.ID, booked.ID)
	}

	res = reservations.Do(http.MethodPost, "/reservations", booking, apitest.Header("Idempotency-Key", "smoke-"+run+"-again"))
	status(t, "book the same period with another Idempotency-Key", res, http.StatusConflict)

	path := "/reservations/" + booked.ID
	later := map[string]any{"startsAt": at(11), "endsAt": at(12)}
	res = reservations.Do(http.MethodPatch, path, later)
	status(t, "reschedule without If-Match", res, http.StatusPreconditionRequired)

	res = reservations.Do(http.MethodPatch, path, later, apitest.Header("If-Match", etag))
	status(t, "reschedule with If-Match", res, http.StatusOK)
	var moved reservation
	res.JSON(t, &moved)
	if starts, err := time.Parse(time.RFC3339, moved.StartsAt); err != nil || !starts.Equal(instant(11)) {
		t.Fatalf("the rescheduled reservation starts at %s, want %s", moved.StartsAt, at(11))
	}

	res = reservations.Do(http.MethodPost, path+"/cancellation", nil, apitest.Header("If-Match", res.Header.Get("ETag")))
	status(t, "cancel the reservation", res, http.StatusOK)
	var cancelled reservation
	res.JSON(t, &cancelled)
	if cancelled.Status != "cancelled" {
		t.Fatalf("the cancelled reservation has the status %q, want cancelled", cancelled.Status)
	}

	// The booking, the rescheduling, and the cancellation each wrote an event
	// to the outbox. The repeated booking and the refused ones wrote none, so
	// the organizer gets three mails, and no more.
	want := []string{"Reservation cancelled", "Reservation confirmed", "Reservation confirmed"}
	got := openMailbox(t).mailTo(t, organizer, len(want))
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Fatalf("the organizer got the mails %q, want %q", got, want)
	}
	t.Logf("ok: the organizer got %d mails through the outbox, the event bus, the queue, and Notification", len(got))
}

// mailbox is Mailpit, which catches the mail of the local system.
type mailbox struct {
	base   *url.URL
	client *http.Client
}

// openMailbox returns Mailpit at the URL in MAILPIT_URL, or at its port in
// compose.yaml.
func openMailbox(t *testing.T) *mailbox {
	t.Helper()
	const fallback = "http://127.0.0.1:18025"
	base := os.Getenv("MAILPIT_URL")
	if base == "" {
		base = fallback
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		t.Fatalf("smoke: MAILPIT_URL=%q is not a URL such as %s", base, fallback)
	}
	return &mailbox{base: u, client: &http.Client{Timeout: 10 * time.Second}}
}

// mailTo waits until Mailpit holds at least n mails to the address, then a
// little longer for a mail too many, and returns what the subjects say before
// their colon, sorted.
func (m *mailbox) mailTo(t *testing.T, address string, n int) []string {
	t.Helper()
	base := m.base.String()
	search := m.base.JoinPath("api", "v1", "search")
	search.RawQuery = url.Values{"query": {`to:"` + address + `"`}}.Encode()
	var subjects []string
	deadline := time.Now().Add(90 * time.Second)
	settled := false
	for {
		found, err := m.subjects(t, search)
		if err != nil {
			t.Fatalf("smoke: Mailpit at %s does not answer: %v\n"+
				"Start the local system with `make up`, or run `make smoke`, which starts it.", base, err)
		}
		subjects = found
		if len(subjects) >= n {
			if settled {
				break
			}
			// Every mail is there. A second delivery of an event would
			// arrive within a few seconds; give it the chance to show.
			settled = true
			time.Sleep(3 * time.Second)
			continue
		}
		if time.Now().After(deadline) {
			t.Fatalf("after 90 seconds, Mailpit holds %d mails to %s, want %d: %q\n"+
				"The relay of Reservation, AWS Emulator, or the worker of Notification dropped an event; see their logs (docker compose logs).",
				len(subjects), address, n, subjects)
		}
		time.Sleep(500 * time.Millisecond)
	}
	for i, subject := range subjects {
		subjects[i], _, _ = strings.Cut(subject, ":")
	}
	sort.Strings(subjects)
	return subjects
}

// subjects returns the subjects of the mails that a search of Mailpit finds.
func (m *mailbox) subjects(t *testing.T, search *url.URL) ([]string, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, search.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s answered %s: %s", search, res.Status, body)
	}
	var found struct {
		Messages []struct {
			Subject string `json:"Subject"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &found); err != nil {
		return nil, fmt.Errorf("GET %s: %w", search, err)
	}
	subjects := make([]string, len(found.Messages))
	for i, m := range found.Messages {
		subjects[i] = m.Subject
	}
	return subjects, nil
}

// connect returns a client of the service at the URL in the environment
// variable, or at its port in compose.yaml, once the service answers its
// health check. Every response that the client receives is checked against
// the API document of the service.
func connect(t *testing.T, service, variable, fallback string) *apitest.Client {
	t.Helper()
	base := os.Getenv(variable)
	if base == "" {
		base = fallback
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		t.Fatalf("smoke: %s=%q is not a URL such as %s", variable, base, fallback)
	}
	// The health check of the image, which the platform serves for every
	// context (httpserver.Main).
	if err := httpserver.CheckHealth(t.Context(), u.Host); err != nil {
		t.Fatalf("smoke: the %s service at %s is not up: %v\n"+
			"Start the local system with `make up`, or run `make smoke`, which starts it.", service, base, err)
	}
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(contracts, service+".openapi.yaml"))
	if err != nil {
		t.Fatalf("smoke: cannot read the API document of %s: %v", service, err)
	}
	return apitest.NewRemote(t, doc, base)
}

// status fails the test when the step did not answer with the status that
// the scenario expects, and logs the step otherwise.
func status(t *testing.T, step string, res apitest.Response, want int) {
	t.Helper()
	if res.Status != want {
		t.Fatalf("%s: got %d, want %d\n%s", step, res.Status, want, res.Body)
	}
	t.Logf("%s: %d", step, res.Status)
}
