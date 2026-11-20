//go:build integration

package app_test

// The acceptance test of the task time-zone-in-mail (evals/) on the side of
// Notification. The grader copies this file into
// backend/internal/notification/internal/app/ and runs it against a database
// of its own. It hands events to the handler of Notification, as the worker
// does, and reads the mail that the in-memory mailer of the tests caught. It
// looks for the times and the name of the time zone in the body, not for a
// whole sentence, so that it does not depend on how the mail is worded. Its
// names start with eval, so that it meets nothing of the tests next to it.

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app/apptest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres/postgrestest"
)

// evalMail hands an event ReservationConfirmed with the data to the handler,
// and returns the body of the one mail that it sent.
func evalMail(t *testing.T, data string) string {
	t.Helper()
	pool := postgrestest.Open(t, "notification")
	mailer := &apptest.Mailer{}
	now := time.Date(2026, time.November, 18, 9, 0, 0, 0, time.UTC)
	notifier := app.NewNotifier(pool, mailer, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	e, err := event.New(uuid.Must(uuid.NewV7()), event.Source("reservation"), "reservation.ReservationConfirmed.v1",
		"0193a6f0-7b5c-7e2a-9c3d-00000000e001", now, json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := notifier.HandleEvent(t.Context(), e); err != nil {
		t.Fatalf("the handler refused the event: %v\n%s", err, data)
	}
	sent := mailer.Sent()
	if len(sent) != 1 {
		t.Fatalf("the event sent %d mails, want 1", len(sent))
	}
	return sent[0].Body
}

// From 01:00 to 02:30 UTC on 20 November 2026, which is from 10:00 to 11:30
// in Tokyo.
const evalPeriod = `"reservationId": "0193a6f0-7b5c-7e2a-9c3d-00000000e001",
	"roomId": "0193a6f0-1a2b-7c3d-8e4f-00000000e002",
	"startsAt": "2026-11-20T01:00:00Z",
	"endsAt": "2026-11-20T02:30:00Z",
	"attendees": 4,
	"organizerEmail": "organizer@example.com"`

func evalWantIn(t *testing.T, what, body string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(body, part) {
			t.Errorf("%s: the mail does not say %q:\n%s", what, part, body)
		}
	}
}

func TestEvalMailInTheTimeZoneOfTheRoom(t *testing.T) {
	t.Parallel()
	body := evalMail(t, `{`+evalPeriod+`, "timeZone": "Asia/Tokyo"}`)
	evalWantIn(t, "an event with the time zone of the room", body, "10:00", "11:30", "Asia/Tokyo")
}

func TestEvalMailOfAnOlderEventInUTC(t *testing.T) {
	t.Parallel()
	body := evalMail(t, `{`+evalPeriod+`}`)
	evalWantIn(t, "an event without a time zone", body, "01:00", "02:30", "UTC")
	if strings.Contains(body, "Asia/Tokyo") {
		t.Errorf("an event without a time zone: the mail names a time zone that the event does not:\n%s", body)
	}
}
