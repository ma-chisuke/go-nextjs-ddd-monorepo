package eventtest_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event/eventtest"
)

// recorder is a testing.TB that keeps the failures instead of failing.
type recorder struct {
	testing.TB
	failures []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func confirmed(t *testing.T, data map[string]any) event.Envelope {
	t.Helper()
	e, err := event.New(uuid.Must(uuid.NewV7()), event.Source("reservation"),
		event.Type("reservation", "ReservationConfirmed", 1), "", time.Now(), data)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func validData() map[string]any {
	return map[string]any{
		"reservationId":  "0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a",
		"roomId":         "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",
		"startsAt":       "2026-11-20T01:00:00Z",
		"endsAt":         "2026-11-20T02:00:00Z",
		"attendees":      4,
		"organizerEmail": "organizer@example.com",
	}
}

func TestAssertMatchesContractAcceptsDataOfTheSchema(t *testing.T) {
	r := &recorder{TB: t}
	eventtest.AssertMatchesContract(r, confirmed(t, validData()))
	if len(r.failures) > 0 {
		t.Errorf("valid data failed: %v", r.failures)
	}
}

func TestAssertMatchesContractFails(t *testing.T) {
	tests := []struct {
		name   string
		change func(data map[string]any)
		want   string
	}{
		{"a required property is missing", func(d map[string]any) { delete(d, "organizerEmail") }, "organizerEmail"},
		{"a property has another type", func(d map[string]any) { d["attendees"] = "four" }, "attendees"},
		{"a format does not hold", func(d map[string]any) { d["startsAt"] = "tomorrow" }, "startsAt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := validData()
			tt.change(data)
			r := &recorder{TB: t}
			eventtest.AssertMatchesContract(r, confirmed(t, data))
			if len(r.failures) != 1 || !strings.Contains(r.failures[0], tt.want) {
				t.Errorf("failures = %v, want one that names %q", r.failures, tt.want)
			}
		})
	}
}

func TestAssertMatchesContractFailsWithoutASchema(t *testing.T) {
	for _, eventType := range []string{"reservation.Unknown.v1", "nocontext", "../reservation.ReservationConfirmed.v1"} {
		e := confirmed(t, validData())
		e.Type = eventType
		r := &recorder{TB: t}
		eventtest.AssertMatchesContract(r, e)
		if len(r.failures) != 1 {
			t.Errorf("type %q: failures = %v, want one", eventType, r.failures)
		}
	}
}
