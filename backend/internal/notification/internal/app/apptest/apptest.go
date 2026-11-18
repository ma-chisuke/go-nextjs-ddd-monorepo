// Package apptest provides an in-memory stand-in for the mailer of the
// Notification context, for tests that must not send mail.
package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
)

// Mailer implements app.Mailer in memory: it keeps each message that it
// sent. The zero value sends every message.
type Mailer struct {
	mu   sync.Mutex
	sent []app.Message
	err  error
	hold *hold
}

var _ app.Mailer = (*Mailer)(nil)

// hold keeps one Send waiting until the test lets it go on.
type hold struct {
	held    chan struct{}
	release chan error
}

// Send keeps m as sent, or fails with the error that Fail set. A Send that
// Hold caught waits first, until the test releases it or ctx ends.
func (m *Mailer) Send(ctx context.Context, msg app.Message) error {
	m.mu.Lock()
	h := m.hold
	m.hold = nil
	err := m.err
	m.mu.Unlock()

	if h != nil {
		close(h.held)
		select {
		case err = <-h.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

// Fail makes every later Send fail with err, and send nothing. A nil err
// makes them send again.
func (m *Mailer) Fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// Hold catches the next Send: held is closed when that Send has started, and
// the Send then waits for release. release(nil) lets it send the message, and
// release(err) makes it fail with err.
func (m *Mailer) Hold() (held <-chan struct{}, release func(error)) {
	h := &hold{held: make(chan struct{}), release: make(chan error, 1)}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hold = h
	return h.held, func(err error) { h.release <- err }
}

// Sent returns the messages that were sent, in order.
func (m *Mailer) Sent() []app.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.sent)
}
