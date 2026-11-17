// Package outbox relays the integration events of a bounded context from its
// outbox table to the broker (TPL-0013).
//
// A context writes each event to its outbox in the transaction that changes
// its state, so an event is stored if and only if the change commits. The
// Relay then reads the outbox and hands each event to a Publisher. When the
// relay stops between the publishing and the mark, the event is published
// again: delivery is at least once, and consumers are idempotent.
//
// The package knows neither the table nor the broker. The context implements
// Source over its own table, and an adapter of the platform implements
// Publisher over the broker.
package outbox

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

// Source is the outbox of one context.
type Source interface {
	// Drain takes up to limit events that are not published yet, the oldest
	// first, and leaves out those that another relay holds at the moment. It
	// calls publish for each one, marks the events for which publish returned
	// nil as published, and returns their number. An event for which publish
	// returned an error stays in the outbox and comes again with a later
	// call. All of it is one transaction: when Drain returns an error, no
	// event is marked.
	Drain(ctx context.Context, limit int, publish func(context.Context, event.Envelope) error) (int, error)
	// Prune deletes the events that were published before the instant, and
	// returns their number.
	Prune(ctx context.Context, before time.Time) (int64, error)
}

// Publisher hands one event to the broker. It returns nil only when the
// broker has taken the event.
type Publisher interface {
	Publish(ctx context.Context, e event.Envelope) error
}

// Config sets the pace of a Relay. A zero field takes its default.
type Config struct {
	// Interval is the pause between two looks at the outbox when it is
	// empty: the longest delay that the relay adds between a commit and the
	// publishing. Default: one second.
	Interval time.Duration
	// Batch is the largest number of events that one transaction publishes.
	// Default: 20.
	Batch int
	// Retention is how long a published event stays in the outbox, for
	// someone who needs to see what was sent. Default: seven days.
	Retention time.Duration
	// PruneInterval is the pause between two removals of old events.
	// Default: one hour.
	PruneInterval time.Duration
}

// The defaults of Config.
const (
	DefaultInterval      = time.Second
	DefaultBatch         = 20
	DefaultRetention     = 7 * 24 * time.Hour
	DefaultPruneInterval = time.Hour
)

func (c Config) withDefaults() Config {
	if c.Interval <= 0 {
		c.Interval = DefaultInterval
	}
	if c.Batch <= 0 {
		c.Batch = DefaultBatch
	}
	if c.Retention <= 0 {
		c.Retention = DefaultRetention
	}
	if c.PruneInterval <= 0 {
		c.PruneInterval = DefaultPruneInterval
	}
	return c
}

// Relay publishes the events of one outbox. Several relays can run on the
// same outbox: each event goes to the one that takes it first.
type Relay struct {
	source    Source
	publisher Publisher
	logger    *slog.Logger
	now       func() time.Time
	config    Config
}

// NewRelay returns the relay from source to publisher. now gives the current
// time.
func NewRelay(source Source, publisher Publisher, logger *slog.Logger, now func() time.Time, config Config) *Relay {
	return &Relay{source: source, publisher: publisher, logger: logger, now: now, config: config.withDefaults()}
}

// Run relays events until ctx is canceled, and then returns nil. A failure of
// the outbox or of the broker is logged, and the relay tries again after the
// interval: the events wait in the outbox meanwhile.
func (r *Relay) Run(ctx context.Context) error {
	r.logger.InfoContext(ctx, "the relay of the outbox runs",
		slog.Duration("interval", r.config.Interval), slog.Int("batch", r.config.Batch))
	tick := time.NewTicker(r.config.Interval)
	defer tick.Stop()
	var pruned time.Time
	for {
		if _, err := r.Flush(ctx); err != nil && ctx.Err() == nil {
			r.logger.ErrorContext(ctx, "the relay could not publish the outbox", slog.Any("error", err))
		}
		if now := r.now(); now.Sub(pruned) >= r.config.PruneInterval && ctx.Err() == nil {
			pruned = now
			r.prune(ctx, now)
		}
		select {
		case <-ctx.Done():
			r.logger.InfoContext(context.WithoutCancel(ctx), "the relay of the outbox stops")
			return nil
		case <-tick.C:
		}
	}
}

// Flush publishes the events that wait in the outbox now, batch after batch,
// and returns how many it published. It stops at the first batch that leaves
// an event behind, and returns the errors of that batch; those events come
// again with the next call.
func (r *Relay) Flush(ctx context.Context) (int, error) {
	total := 0
	for {
		var failures []error
		published, err := r.source.Drain(ctx, r.config.Batch, func(ctx context.Context, e event.Envelope) error {
			if err := r.publisher.Publish(ctx, e); err != nil {
				failures = append(failures, err)
				r.logger.WarnContext(ctx, "an event was not published and stays in the outbox",
					slog.String("event_id", e.ID.String()), slog.String("event_type", e.Type), slog.Any("error", err))
				return err
			}
			r.logger.InfoContext(ctx, "an event was published",
				slog.String("event_id", e.ID.String()), slog.String("event_type", e.Type))
			return nil
		})
		if err != nil {
			// Nothing of this batch is marked, so its events are published
			// again: at least once.
			return total, err
		}
		total += published
		if len(failures) > 0 {
			return total, errors.Join(failures...)
		}
		if published < r.config.Batch {
			return total, nil
		}
	}
}

func (r *Relay) prune(ctx context.Context, now time.Time) {
	n, err := r.source.Prune(ctx, now.Add(-r.config.Retention))
	if err != nil {
		if ctx.Err() == nil {
			r.logger.ErrorContext(ctx, "the relay could not remove old events from the outbox", slog.Any("error", err))
		}
		return
	}
	if n > 0 {
		r.logger.InfoContext(ctx, "old events were removed from the outbox", slog.Int64("events", n))
	}
}
