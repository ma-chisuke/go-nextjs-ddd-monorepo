package event_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

var (
	id = uuid.MustParse("0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a")
	at = time.Date(2026, 11, 20, 10, 0, 0, 0, time.FixedZone("JST", 9*60*60))
)

func TestNewMakesACloudEvent(t *testing.T) {
	e, err := event.New(id, event.Source("reservation"), event.Type("reservation", "ReservationConfirmed", 1),
		"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6b", at, map[string]int{"attendees": 4})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"specversion":"1.0","id":"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a","source":"/contexts/reservation",` +
		`"type":"reservation.ReservationConfirmed.v1","time":"2026-11-20T01:00:00Z",` +
		`"subject":"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6b","datacontenttype":"application/json","data":{"attendees":4}}`
	if string(raw) != want {
		t.Errorf("the envelope is\n%s\nwant\n%s", raw, want)
	}
}

func TestParseReturnsWhatNewMade(t *testing.T) {
	made, err := event.New(id, "/contexts/reservation", "reservation.ReservationCancelled.v1", "", at, struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(made)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := event.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID != made.ID || parsed.Source != made.Source || parsed.Type != made.Type ||
		!parsed.Time.Equal(made.Time) || string(parsed.Data) != string(made.Data) {
		t.Errorf("Parse = %+v, want %+v", parsed, made)
	}
}

func TestNewRefusesDataThatIsNotJSON(t *testing.T) {
	if _, err := event.New(id, "/contexts/reservation", "reservation.X.v1", "", at, func() {}); err == nil {
		t.Error("New took data that cannot be marshaled")
	}
}

func TestParseRefusesWhatIsNotAnEnvelope(t *testing.T) {
	const valid = `{"specversion":"1.0","id":"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a","source":"/contexts/reservation",` +
		`"type":"reservation.ReservationConfirmed.v1","time":"2026-11-20T01:00:00Z","datacontenttype":"application/json","data":{}}`
	if _, err := event.Parse([]byte(valid)); err != nil {
		t.Fatalf("the valid envelope was refused: %v", err)
	}
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"not JSON", `{`, "not an envelope"},
		{"another specversion", strings.Replace(valid, `"1.0"`, `"0.3"`, 1), "specversion"},
		{"no id", strings.Replace(valid, `"id":"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a",`, "", 1), "id is missing"},
		{"an id that is no UUID", strings.Replace(valid, "0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a", "42", 1), "not an envelope"},
		{"no source", strings.Replace(valid, `"source":"/contexts/reservation",`, "", 1), "source is missing"},
		{"no type", strings.Replace(valid, `"type":"reservation.ReservationConfirmed.v1",`, "", 1), "type is missing"},
		{"no time", strings.Replace(valid, `"time":"2026-11-20T01:00:00Z",`, "", 1), "time is missing"},
		{"another content type", strings.Replace(valid, "application/json", "text/plain", 1), "datacontenttype"},
		{"no data", strings.Replace(valid, `,"data":{}`, "", 1), "data is not JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := event.Parse([]byte(tt.raw))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Parse = %v, want an error that contains %q", err, tt.want)
			}
		})
	}
}
