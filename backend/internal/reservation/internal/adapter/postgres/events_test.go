package postgres

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event/eventtest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

var (
	sampleZone        = time.FixedZone("Asia/Tokyo", 9*60*60)
	sampleReservation = domain.ReservationID(uuid.MustParse("0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a"))
	sampleRoom        = domain.RoomID(uuid.MustParse("0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f"))
	// The period from 10:00 to 11:00 in Tokyo, which is 01:00 to 02:00 UTC.
	samplePeriod = domain.Period{
		Start: time.Date(2026, time.November, 20, 10, 0, 0, 0, sampleZone),
		End:   time.Date(2026, time.November, 20, 11, 0, 0, 0, sampleZone),
	}
	sampleTime = time.Date(2026, time.November, 18, 10, 0, 0, 0, sampleZone)
)

// eventSamples holds one domain event of each type that the domain makes,
// by the name of its type, and the data of the integration event that
// publishes it.
var eventSamples = map[string]struct {
	event domain.Event
	data  string
}{
	"ReservationConfirmed": {
		event: domain.ReservationConfirmed{
			Reservation: sampleReservation, Room: sampleRoom, Period: samplePeriod, Attendees: 4,
			Organizer: "organizer@example.com", OccurredAt: sampleTime,
		},
		data: `{"reservationId":"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a","roomId":"0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",` +
			`"startsAt":"2026-11-20T01:00:00Z","endsAt":"2026-11-20T02:00:00Z","attendees":4,"organizerEmail":"organizer@example.com"}`,
	},
	"ReservationCancelled": {
		event: domain.ReservationCancelled{
			Reservation: sampleReservation, Room: sampleRoom, Period: samplePeriod,
			Organizer: "organizer@example.com", OccurredAt: sampleTime,
		},
		data: `{"reservationId":"0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a","roomId":"0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",` +
			`"startsAt":"2026-11-20T01:00:00Z","endsAt":"2026-11-20T02:00:00Z","organizerEmail":"organizer@example.com"}`,
	},
}

// Every type of domain event has a sample above, so the test below passes
// each one through the mapping. A new type of domain event fails here until
// it has a sample, and then fails the test below until it has a mapping and a
// contract.
func TestEverySampleOfADomainEvent(t *testing.T) {
	t.Parallel()
	got := domainEventTypes(t)
	want := slices.Sorted(maps.Keys(eventSamples))
	if !slices.Equal(got, want) {
		t.Errorf("the domain makes the events %v, and the samples of this test are %v: add a sample, a mapping in integrationEvent, and a contract for each new one", got, want)
	}
}

func TestIntegrationEventsMatchTheirContracts(t *testing.T) {
	t.Parallel()
	for name, sample := range eventSamples {
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatal(err)
		}
		e, err := integrationEvent(id, sample.event)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		eventtest.AssertMatchesContract(t, e)
		if want := "reservation." + name + ".v1"; e.Type != want {
			t.Errorf("%s: the type is %q, want %q", name, e.Type, want)
		}
		if e.ID != id || e.Source != "/contexts/reservation" || e.Subject != sampleReservation.String() ||
			!e.Time.Equal(sampleTime) || e.Time.Location() != time.UTC {
			t.Errorf("%s: got the envelope %+v, want the id %s, the source /contexts/reservation, the reservation as subject, and the time of the change in UTC", name, e, id)
		}
		if string(e.Data) != sample.data {
			t.Errorf("%s: got the data\n%s\nwant\n%s", name, e.Data, sample.data)
		}
		// What the outbox stores and the relay reads back is the same event.
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := event.Parse(raw)
		if err != nil {
			t.Errorf("%s: the stored envelope does not parse: %v", name, err)
		}
		if parsed.ID != e.ID || parsed.Type != e.Type || !parsed.Time.Equal(e.Time) || string(parsed.Data) != string(e.Data) {
			t.Errorf("%s: got %+v back from the outbox, want %+v", name, parsed, e)
		}
	}
}

// A domain event without a mapping is an error, never dropped.
func TestADomainEventWithoutAMappingFails(t *testing.T) {
	t.Parallel()
	for _, e := range []domain.Event{nil, &domain.ReservationConfirmed{Reservation: sampleReservation}} {
		if got, err := integrationEvent(uuid.New(), e); err == nil {
			t.Errorf("the domain event %#v became %+v, want an error", e, got)
		}
	}
}

// domainEventTypes returns the names of the types that implement domain.Event,
// in order: those with the method event, which only the package domain can
// declare.
func domainEventTypes(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "domain", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "event" || len(fn.Recv.List) != 1 {
				continue
			}
			receiver := fn.Recv.List[0].Type
			if star, ok := receiver.(*ast.StarExpr); ok {
				receiver = star.X
			}
			if ident, ok := receiver.(*ast.Ident); ok {
				names = append(names, ident.Name)
			}
		}
	}
	if len(names) == 0 {
		t.Fatalf("no type in %v has the method event of domain.Event", files)
	}
	slices.Sort(names)
	return names
}
