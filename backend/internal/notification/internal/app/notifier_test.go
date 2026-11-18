package app_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app/apptest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

var now = time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)

// newNotifier returns the use case without a database: these tests end
// before the use case reaches it. The log goes to the returned buffer.
func newNotifier(t *testing.T) (*app.Notifier, *apptest.Mailer, *bytes.Buffer) {
	t.Helper()
	mailer := &apptest.Mailer{}
	var log bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&log, nil))
	return app.NewNotifier(nil, mailer, func() time.Time { return now }, logger), mailer, &log
}

func envelope(t *testing.T, eventType, data string) event.Envelope {
	t.Helper()
	e, err := event.New(uuid.Must(uuid.NewV7()), event.Source("reservation"), eventType,
		"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a", now, json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestAnEventOfAnotherTypeIsIgnored(t *testing.T) {
	t.Parallel()
	for _, eventType := range []string{
		"reservation.ReservationRescheduled.v1",
		"reservation.ReservationConfirmed.v2",
		"facility.RoomRegistered.v1",
	} {
		t.Run(eventType, func(t *testing.T) {
			t.Parallel()
			notifier, mailer, log := newNotifier(t)
			if err := notifier.HandleEvent(t.Context(), envelope(t, eventType, `{"anything": true}`)); err != nil {
				t.Errorf("HandleEvent = %v, want nil: the event is handled by being ignored", err)
			}
			if sent := mailer.Sent(); len(sent) != 0 {
				t.Errorf("%d mails were sent, want none", len(sent))
			}
			if !strings.Contains(log.String(), "does not handle") || !strings.Contains(log.String(), eventType) {
				t.Errorf("the log does not say that the event was ignored:\n%s", log)
			}
		})
	}
}

// An event of a known type whose data breaks its contract fails, so that it
// comes again and, after the retries, goes to the dead-letter queue.
func TestDataThatBreaksTheContractIsAnError(t *testing.T) {
	t.Parallel()
	const confirmed = "reservation.ReservationConfirmed.v1"
	const cancelled = "reservation.ReservationCancelled.v1"
	valid := map[string]any{
		"reservationId":  "0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a",
		"roomId":         "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",
		"startsAt":       "2026-11-20T01:00:00Z",
		"endsAt":         "2026-11-20T02:00:00Z",
		"attendees":      4,
		"organizerEmail": "organizer@example.com",
	}
	with := func(key string, value any) string {
		data := map[string]any{}
		for k, v := range valid {
			data[k] = v
		}
		if value == nil {
			delete(data, key)
		} else {
			data[key] = value
		}
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	tests := []struct {
		name      string
		eventType string
		data      string
		want      string
	}{
		{"not an object", confirmed, `[]`, "cannot be read"},
		{"a reservation id that is no UUID", confirmed, with("reservationId", "42"), "cannot be read"},
		{"no reservation id", cancelled, with("reservationId", nil), "reservationId is missing"},
		{"no room id", confirmed, with("roomId", nil), "roomId is missing"},
		{"no start", cancelled, with("startsAt", nil), "startsAt is missing"},
		{"a start that is no date-time", confirmed, with("startsAt", "tomorrow"), "cannot be read"},
		{"no end", confirmed, with("endsAt", nil), "endsAt is missing"},
		{"no attendees", confirmed, with("attendees", nil), "attendees is missing"},
		{"attendees as text", confirmed, with("attendees", "four"), "cannot be read"},
		{"no organizer", cancelled, with("organizerEmail", nil), "organizerEmail"},
		{"an organizer that is no address", confirmed, with("organizerEmail", "the organizer"), "organizerEmail"},
		{"two addresses", confirmed, with("organizerEmail", "a@example.com, b@example.com"), "organizerEmail"},
		{"a line break in the address", cancelled, with("organizerEmail", "a@example.com\r\nBcc: b@example.com"), "organizerEmail"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			notifier, mailer, _ := newNotifier(t)
			err := notifier.HandleEvent(t.Context(), envelope(t, tt.eventType, tt.data))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("HandleEvent = %v, want an error that contains %q", err, tt.want)
			}
			if sent := mailer.Sent(); len(sent) != 0 {
				t.Errorf("%d mails were sent, want none", len(sent))
			}
		})
	}
}
