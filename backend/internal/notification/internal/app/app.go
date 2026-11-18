// Package app holds the use case of the Notification context: it handles the
// events of Reservation and sends mail to the organizer.
//
// Notification is a supporting subdomain: it has one rule, send each mail
// once. So its use case is a transaction script that calls the generated
// queries directly, and the context has no domain layer (TPL-0003).
package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

// commitTimeout bounds the commit after a mail was sent. The commit does not
// end with the context of the event: once the mail is out, losing its record
// would send it twice.
const commitTimeout = 10 * time.Second

// Notifier turns the events of Reservation into mail to the organizer, one
// mail for each event however often the event is delivered.
type Notifier struct {
	pool   *pgxpool.Pool
	mailer Mailer
	now    func() time.Time
	logger *slog.Logger
}

// NewNotifier returns the use case. pool connects as the runtime role of
// Notification, and now gives the current time.
func NewNotifier(pool *pgxpool.Pool, mailer Mailer, now func() time.Time, logger *slog.Logger) *Notifier {
	return &Notifier{pool: pool, mailer: mailer, now: now, logger: logger}
}

// HandleEvent handles one event of another context, and is the event.Handler
// of the context. It sends the mail that an event of a known type asks for,
// unless it sent the mail of that event before, and ignores an event of any
// other type. It returns an error when the data of the event cannot be read,
// or when the mail or the database fails: the event then comes again.
func (n *Notifier) HandleEvent(ctx context.Context, e event.Envelope) error {
	m, known, err := compose(e)
	if err != nil {
		return fmt.Errorf("notification: event %s of type %s: %w", e.ID, e.Type, err)
	}
	if !known {
		n.logger.InfoContext(ctx, "an event of a type that Notification does not handle was ignored",
			slog.String("event_id", e.ID.String()), slog.String("event_type", e.Type))
		return nil
	}
	sent, err := n.sendOnce(ctx, e, m)
	if err != nil {
		return fmt.Errorf("notification: event %s of type %s: %w", e.ID, e.Type, err)
	}
	if sent {
		n.logger.InfoContext(ctx, "a mail was sent",
			slog.String("event_id", e.ID.String()), slog.String("event_type", e.Type))
	} else {
		n.logger.InfoContext(ctx, "the mail of the event was sent before, so none was sent again",
			slog.String("event_id", e.ID.String()), slog.String("event_type", e.Type))
	}
	return nil
}

// sendOnce sends m, unless a mail of the event is recorded already, and
// records it, in one transaction. It reports whether it sent the mail.
//
// The record comes first, under the unique constraint on the event id. When
// another delivery of the same event holds that record in a transaction that
// has not ended, the insert waits for it: after its commit, the insert finds
// the record and nothing is sent; after its rollback, the insert succeeds and
// this delivery sends the mail. When the mail fails, the record rolls back,
// and the event can be handled again.
func (n *Notifier) sendOnce(ctx context.Context, e event.Envelope, m Message) (sent bool, err error) {
	tx, err := n.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	// After the commit, the rollback does nothing. Before it, every return
	// takes the record back; if ctx has ended, pgx closes the connection,
	// and PostgreSQL rolls back.
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := postgres.New(tx).InsertNotification(ctx, postgres.InsertNotificationParams{
		EventID:        e.ID,
		EventType:      e.Type,
		RecipientEmail: m.To,
		Subject:        m.Subject,
		SentAt:         n.now(),
	})
	if err != nil {
		return false, fmt.Errorf("record the mail: %w", err)
	}
	if rows == 0 {
		return false, nil
	}
	if err := n.mailer.Send(ctx, m); err != nil {
		return false, fmt.Errorf("send the mail: %w", err)
	}

	// The one window that delivery at least once leaves open: the mail is
	// out, and if the commit fails now, the record is gone, the event comes
	// again, and the organizer gets the mail twice. Delivery at least once
	// cannot avoid that without the help of the mail server, which would have
	// to recognize the mail when it comes again. The commit therefore does
	// not end with ctx, which keeps the window as short as the commit.
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commitTimeout)
	defer cancel()
	if err := tx.Commit(commitCtx); err != nil {
		return true, fmt.Errorf("the mail was sent, but its record was not committed: %w", err)
	}
	return true, nil
}
