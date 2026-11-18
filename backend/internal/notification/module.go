// Package notification is the Notification bounded context: it handles the
// events of Reservation and sends mail to the organizer. See
// docs/contexts/notification/.
//
// This package wires the context together, and only cmd/notification uses
// it. Notification has no API: other contexts reach it only through the
// events that they publish (TPL-0005, TPL-0013).
package notification

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/adapter/smtp"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

// Deps is what the context needs from the process that runs it.
type Deps struct {
	// Pool connects to the database as the runtime role of Notification.
	Pool *pgxpool.Pool
	// Logger receives what the context did with each event.
	Logger *slog.Logger
	// Now gives the current time.
	Now func() time.Time
	// SMTPAddr is the host and port of the mail server, such as
	// mailpit:1025. Notification speaks plain SMTP to it, without
	// authentication or TLS.
	SMTPAddr string
	// MailFrom is the address that the mail comes from, such as
	// "Meeting rooms <rooms@example.com>".
	MailFrom string
}

// NewHandler returns the handler of the events that Notification receives.
// It fails when the settings of the mail server are not complete.
func NewHandler(deps Deps) (event.Handler, error) {
	mailer, err := smtp.New(smtp.Config{Addr: deps.SMTPAddr, From: deps.MailFrom, Now: deps.Now})
	if err != nil {
		return nil, err
	}
	return app.NewNotifier(deps.Pool, mailer, deps.Now, deps.Logger).HandleEvent, nil
}
