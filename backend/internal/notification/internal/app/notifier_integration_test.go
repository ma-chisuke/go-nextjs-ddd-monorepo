//go:build integration

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app/apptest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres/postgrestest"
)

// notifierInTheDatabase returns the use case over the real database,
// connected as the runtime role of Notification, with a mailer in memory.
func notifierInTheDatabase(t *testing.T) (*app.Notifier, *apptest.Mailer, *pgxpool.Pool) {
	t.Helper()
	pool := postgrestest.Open(t, "notification")
	mailer := &apptest.Mailer{}
	notifier := app.NewNotifier(pool, mailer, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	return notifier, mailer, pool
}

// confirmed returns a new event ReservationConfirmed. Its record is deleted
// when the test ends.
func confirmed(t *testing.T, pool *pgxpool.Pool) event.Envelope {
	t.Helper()
	e := envelope(t, "reservation.ReservationConfirmed.v1", `{
		"reservationId": "0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a",
		"roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",
		"startsAt": "2026-11-20T01:00:00Z",
		"endsAt": "2026-11-20T02:00:00Z",
		"attendees": 4,
		"organizerEmail": "organizer@example.com"
	}`)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM notifications WHERE event_id = $1", e.ID); err != nil {
			t.Errorf("cannot delete the record of the test: %v", err)
		}
	})
	return e
}

// record is the row of the notifications table for an event.
type record struct {
	eventType, recipient, subject string
	sentAt                        time.Time
}

// records returns the rows for the event.
func records(t *testing.T, pool *pgxpool.Pool, e event.Envelope) []record {
	t.Helper()
	rows, err := pool.Query(t.Context(),
		"SELECT event_type, recipient_email, subject, sent_at FROM notifications WHERE event_id = $1", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var found []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.eventType, &r.recipient, &r.subject, &r.sentAt); err != nil {
			t.Fatal(err)
		}
		found = append(found, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return found
}

func wantOneMail(t *testing.T, mailer *apptest.Mailer, pool *pgxpool.Pool, e event.Envelope) {
	t.Helper()
	sent := mailer.Sent()
	if len(sent) != 1 {
		t.Fatalf("%d mails were sent, want 1", len(sent))
	}
	found := records(t, pool, e)
	if len(found) != 1 {
		t.Fatalf("the event has %d records, want 1", len(found))
	}
	want := record{eventType: e.Type, recipient: sent[0].To, subject: sent[0].Subject, sentAt: now}
	if got := found[0]; got.eventType != want.eventType || got.recipient != want.recipient ||
		got.subject != want.subject || !got.sentAt.Equal(want.sentAt) {
		t.Errorf("the record is %+v, want %+v", got, want)
	}
}

func TestAnEventSendsOneMailAndRecordsIt(t *testing.T) {
	t.Parallel()
	notifier, mailer, pool := notifierInTheDatabase(t)
	e := confirmed(t, pool)

	if err := notifier.HandleEvent(t.Context(), e); err != nil {
		t.Fatal(err)
	}
	wantOneMail(t, mailer, pool, e)
	if to := mailer.Sent()[0].To; to != "organizer@example.com" {
		t.Errorf("the mail went to %q, want the organizer", to)
	}
}

// Delivery is at least once: the same event again sends nothing.
func TestTheSameEventAgainSendsNothing(t *testing.T) {
	t.Parallel()
	notifier, mailer, pool := notifierInTheDatabase(t)
	e := confirmed(t, pool)

	for range 3 {
		if err := notifier.HandleEvent(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	wantOneMail(t, mailer, pool, e)
}

// A mail that fails leaves no record, so the event, which comes again,
// sends it then.
func TestAFailedMailLeavesNoRecord(t *testing.T) {
	t.Parallel()
	notifier, mailer, pool := notifierInTheDatabase(t)
	e := confirmed(t, pool)
	down := errors.New("the mail server is down")

	mailer.Fail(down)
	if err := notifier.HandleEvent(t.Context(), e); !errors.Is(err, down) {
		t.Fatalf("HandleEvent = %v, want the error of the mailer", err)
	}
	if found := records(t, pool, e); len(found) != 0 {
		t.Fatalf("the failed mail left %d records, want none", len(found))
	}

	mailer.Fail(nil)
	if err := notifier.HandleEvent(t.Context(), e); err != nil {
		t.Fatal(err)
	}
	wantOneMail(t, mailer, pool, e)
}

// Two deliveries of one event at the same moment send one mail: the insert of
// the second waits for the transaction of the first. The two cases run one
// after the other, because each looks for a waiting insert of its own.
func TestConcurrentDeliveriesSendOneMail(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		// first is how the mail of the first delivery ends.
		first error
	}{
		{name: "the first delivery sends, so the second sends nothing", first: nil},
		{name: "the first delivery fails, so the second sends", first: errors.New("the mail server is down")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			notifier, mailer, pool := notifierInTheDatabase(t)
			e := confirmed(t, pool)
			held, release := mailer.Hold()

			first := make(chan error, 1)
			go func() { first <- notifier.HandleEvent(t.Context(), e) }()
			// The first delivery has recorded the event, and is sending.
			<-held
			second := make(chan error, 1)
			go func() { second <- notifier.HandleEvent(t.Context(), e) }()
			waitForAWaitingInsert(t, pool)
			release(tt.first)

			if err := <-first; !errors.Is(err, tt.first) {
				t.Errorf("the first delivery returned %v, want %v", err, tt.first)
			}
			if err := <-second; err != nil {
				t.Errorf("the second delivery returned %v, want nil", err)
			}
			wantOneMail(t, mailer, pool, e)
		})
	}
}

// waitForAWaitingInsert returns when a session of the runtime role waits for
// a lock in an insert into notifications.
func waitForAWaitingInsert(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	const query = `SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database() AND usename = current_user AND pid <> pg_backend_pid()
		AND wait_event_type = 'Lock' AND query LIKE '%INSERT INTO notifications (%'`
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(t.Context(), query).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the second delivery did not wait for the first within 10 seconds")
}

// The runtime role works with the rows of its own schema, and with nothing
// else (TPL-0005, TPL-0009).
func TestRuntimeRolePrivileges(t *testing.T) {
	t.Parallel()
	pool := postgrestest.Open(t, "notification")
	denied := map[string]string{
		"create a table in its schema":        "CREATE TABLE notification.intruder (id integer)",
		"change a table":                      "ALTER TABLE notifications ADD COLUMN intruder integer",
		"drop a table":                        "DROP TABLE notifications",
		"empty a table":                       "TRUNCATE notifications",
		"read the schema of Reservation":      "SELECT 1 FROM reservation.reservations",
		"read the schema of Facility":         "SELECT 1 FROM facility.rooms",
		"create a table in the public schema": "CREATE TABLE public.intruder (id integer)",
	}
	for what, statement := range denied {
		if _, err := pool.Exec(t.Context(), statement); err == nil {
			t.Errorf("the runtime role of Notification could %s", what)
		}
	}
}
