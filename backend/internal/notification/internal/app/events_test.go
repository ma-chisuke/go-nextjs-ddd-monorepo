package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event/eventtest"
)

var at = time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)

func envelope(t *testing.T, eventType string, data string) event.Envelope {
	t.Helper()
	e, err := event.New(uuid.Must(uuid.NewV7()), event.Source("reservation"), eventType,
		"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a", at, json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestTheTextOfEachMail(t *testing.T) {
	t.Parallel()
	// Times with an offset, to show that the mail writes them in UTC.
	const period = `"reservationId": "0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a",
		"roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",
		"startsAt": "2026-11-20T10:00:00+09:00",
		"endsAt": "2026-11-20T11:00:00+09:00",
		"organizerEmail": "organizer@example.com"`
	tests := []struct {
		eventType string
		data      string
		want      Message
	}{
		{
			eventType: "reservation.ReservationConfirmed.v1",
			data:      `{` + period + `, "attendees": 4}`,
			want: Message{
				To:      "organizer@example.com",
				Subject: "Reservation confirmed: Fri, 20 Nov 2026 01:00 UTC",
				Body: "Your reservation of a meeting room is confirmed.\n" +
					"If you rescheduled it, this period replaces the one before.\n" +
					"\n" +
					"Reservation: 0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a\n" +
					"Room:        0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f\n" +
					"From:        Fri, 20 Nov 2026 01:00 UTC\n" +
					"Until:       Fri, 20 Nov 2026 02:00 UTC\n" +
					"Attendees:   4\n",
			},
		},
		{
			eventType: "reservation.ReservationCancelled.v1",
			data:      `{` + period + `}`,
			want: Message{
				To:      "organizer@example.com",
				Subject: "Reservation cancelled: Fri, 20 Nov 2026 01:00 UTC",
				Body: "Your reservation of a meeting room is cancelled.\n" +
					"The room is free again for this period.\n" +
					"\n" +
					"Reservation: 0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a\n" +
					"Room:        0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f\n" +
					"From:        Fri, 20 Nov 2026 01:00 UTC\n" +
					"Until:       Fri, 20 Nov 2026 02:00 UTC\n",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			t.Parallel()
			got, known, err := compose(envelope(t, tt.eventType, tt.data))
			if err != nil || !known {
				t.Fatalf("compose = %v, known %t, want a mail", err, known)
			}
			if got != tt.want {
				t.Errorf("the mail is\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

// Every example in the JSON Schema of each event that Notification handles
// is an event that it turns into a mail. The test reads the contract, so a
// change to its examples reaches it.
func TestEachExampleOfTheContractMakesAMail(t *testing.T) {
	t.Parallel()
	for _, eventType := range []string{typeReservationConfirmed, typeReservationCancelled} {
		path, err := eventtest.SchemaPath(eventType)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path) //nolint:gosec // The path is that of a contract in the repository.
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Examples []json.RawMessage `json:"examples"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		if len(schema.Examples) == 0 {
			t.Errorf("%s has no examples", path)
		}
		for i, example := range schema.Examples {
			e := envelope(t, eventType, string(example))
			// The example is an event that Reservation may publish.
			eventtest.AssertMatchesContract(t, e)
			var want struct {
				ReservationID  string    `json:"reservationId"`
				RoomID         string    `json:"roomId"`
				StartsAt       time.Time `json:"startsAt"`
				OrganizerEmail string    `json:"organizerEmail"`
			}
			if err := json.Unmarshal(example, &want); err != nil {
				t.Fatal(err)
			}
			m, known, err := compose(e)
			if err != nil || !known {
				t.Errorf("%s, example %d: compose = %v, known %t, want a mail", eventType, i, err, known)
				continue
			}
			if m.To != want.OrganizerEmail {
				t.Errorf("%s, example %d: the mail goes to %q, want the organizer %q", eventType, i, m.To, want.OrganizerEmail)
			}
			for _, part := range []string{want.ReservationID, want.RoomID, instant(want.StartsAt)} {
				if !strings.Contains(m.Body, part) {
					t.Errorf("%s, example %d: the mail does not say %q:\n%s", eventType, i, part, m.Body)
				}
			}
		}
	}
}

// Reservation may add properties to its events; Notification reads the ones
// it knows, and takes the bare address out of an address with a name.
func TestTheDataMayCarryMore(t *testing.T) {
	t.Parallel()
	data := `{
		"reservationId": "0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a",
		"roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",
		"startsAt": "2026-11-20T01:00:00Z",
		"endsAt": "2026-11-20T02:00:00Z",
		"organizerEmail": "Ann Organizer <ann@example.com>",
		"roomName": "Aoba",
		"organizer": {"name": "Ann"}
	}`
	m, known, err := compose(envelope(t, typeReservationCancelled, data))
	if err != nil || !known {
		t.Fatalf("compose = %v, known %t, want a mail", err, known)
	}
	if m.To != "ann@example.com" {
		t.Errorf("the mail goes to %q, want ann@example.com", m.To)
	}
}
