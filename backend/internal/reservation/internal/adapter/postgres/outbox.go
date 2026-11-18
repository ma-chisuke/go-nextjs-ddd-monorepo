package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/outbox"
)

// Outbox implements outbox.Source over the table outbox, connected as the
// runtime role of Reservation. The store writes each event there in the
// transaction of its change (Store.Add, Store.Update), and the relay of the
// platform reads it from here (TPL-0013).
type Outbox struct {
	pool    *pgxpool.Pool
	queries *Queries
	now     func() time.Time
	logger  *slog.Logger
}

var _ outbox.Source = (*Outbox)(nil)

// NewOutbox returns the outbox over pool. now gives the instant at which
// Drain marks an event as published, and logger receives the events that
// cannot be read.
func NewOutbox(pool *pgxpool.Pool, now func() time.Time, logger *slog.Logger) *Outbox {
	return &Outbox{pool: pool, queries: New(pool), now: now, logger: logger}
}

// Drain implements outbox.Source. It locks up to limit events that wait, the
// oldest first, with FOR UPDATE SKIP LOCKED: the events that another relay
// holds are left out, and another relay leaves out these, so no event goes
// to two relays at once. It calls publish for each one, then marks the events
// for which publish returned nil, and commits. When Drain returns an error,
// the transaction rolled back and no event is marked.
//
// A row that does not hold a valid envelope under its own id is a bug of the
// code that wrote it. Drain does not hand it to publish: it logs the error,
// leaves the row in the outbox like an event that publish refused, and goes
// on with the other events, so that one bad row does not hold up the rest.
func (o *Outbox) Drain(ctx context.Context, limit int, publish func(context.Context, event.Envelope) error) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	batch, err := int32Of(limit)
	if err != nil {
		return 0, err
	}
	var published []uuid.UUID
	err = pgx.BeginFunc(ctx, o.pool, func(tx pgx.Tx) error {
		q := o.queries.WithTx(tx)
		rows, err := q.LockUnpublishedEvents(ctx, batch)
		if err != nil {
			return err
		}
		for _, row := range rows {
			e, err := envelopeOf(row)
			if err != nil {
				o.logger.ErrorContext(ctx, "an event of the outbox cannot be read, and stays in the outbox",
					slog.String("event_id", row.ID.String()), slog.Any("error", err))
				continue
			}
			// The relay logs the error of publish, and the event stays.
			if publish(ctx, e) != nil {
				continue
			}
			published = append(published, row.ID)
		}
		if len(published) == 0 {
			return nil
		}
		marked, err := q.MarkEventsPublished(ctx, MarkEventsPublishedParams{PublishedAt: o.now(), Ids: published})
		if err != nil {
			return err
		}
		if marked != int64(len(published)) {
			return fmt.Errorf("%d events were published, and %d were marked", len(published), marked)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("drain the outbox: %w", err)
	}
	return len(published), nil
}

// envelopeOf returns the envelope that a row of the outbox holds.
func envelopeOf(row LockUnpublishedEventsRow) (event.Envelope, error) {
	e, err := event.Parse(row.Payload)
	if err != nil {
		return event.Envelope{}, err
	}
	if e.ID != row.ID {
		return event.Envelope{}, fmt.Errorf("the row %s holds the event %s", row.ID, e.ID)
	}
	return e, nil
}

// Prune implements outbox.Source.
func (o *Outbox) Prune(ctx context.Context, before time.Time) (int64, error) {
	n, err := o.queries.DeletePublishedEvents(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("prune the outbox: %w", err)
	}
	return n, nil
}
