// Package outboxtest holds fakes of the ports of package outbox, for tests of
// the relay and of the code that runs one.
package outboxtest

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/outbox"
)

// Source is an outbox in memory. It keeps the contract of outbox.Source,
// except that it has no second relay to leave events to.
type Source struct {
	mu     sync.Mutex
	events []stored
	// Err, when set, is what Drain and Prune return, as an outbox whose
	// database is away.
	Err error
	// Now gives the instant that Drain marks events with. Default: time.Now.
	Now func() time.Time
}

type stored struct {
	event       event.Envelope
	publishedAt time.Time
}

var _ outbox.Source = (*Source)(nil)

// Add puts events into the outbox, as a transaction of the context would.
func (s *Source) Add(events ...event.Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range events {
		s.events = append(s.events, stored{event: e})
	}
}

// Pending returns the ids of the events that are not published, the oldest
// first.
func (s *Source) Pending() []uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []uuid.UUID
	for _, e := range s.events {
		if e.publishedAt.IsZero() {
			ids = append(ids, e.event.ID)
		}
	}
	return ids
}

// Len returns the number of events in the outbox, published or not.
func (s *Source) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

// Drain implements outbox.Source.
func (s *Source) Drain(ctx context.Context, limit int, publish func(context.Context, event.Envelope) error) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return 0, s.Err
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	taken, published := 0, 0
	for i := range s.events {
		if taken == limit {
			break
		}
		if !s.events[i].publishedAt.IsZero() {
			continue
		}
		taken++
		if err := publish(ctx, s.events[i].event); err != nil {
			continue
		}
		s.events[i].publishedAt = now()
		published++
	}
	return published, nil
}

// Prune implements outbox.Source.
func (s *Source) Prune(_ context.Context, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return 0, s.Err
	}
	kept := s.events[:0]
	var n int64
	for _, e := range s.events {
		if !e.publishedAt.IsZero() && e.publishedAt.Before(before) {
			n++
			continue
		}
		kept = append(kept, e)
	}
	s.events = kept
	return n, nil
}

// Publisher is a broker in memory: it keeps what it was handed.
type Publisher struct {
	mu        sync.Mutex
	published []event.Envelope
	// Refuse, when set, decides which events the broker refuses.
	Refuse func(e event.Envelope) bool
}

var _ outbox.Publisher = (*Publisher)(nil)

// ErrRefused is what Publish returns for an event that Refuse selects.
var ErrRefused = errors.New("the broker refused the event")

// Publish implements outbox.Publisher.
func (p *Publisher) Publish(_ context.Context, e event.Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Refuse != nil && p.Refuse(e) {
		return ErrRefused
	}
	p.published = append(p.published, e)
	return nil
}

// Published returns the events that the broker took, in the order of their
// arrival.
func (p *Publisher) Published() []event.Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]event.Envelope(nil), p.published...)
}
