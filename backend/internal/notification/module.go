// Package notification is the Notification bounded context: it handles the
// events of Reservation and sends mail to the organizer. See
// docs/contexts/notification/.
//
// This package wires the context together, and only cmd/notification uses
// it. Notification has no API: other contexts reach it only through the
// events that they publish (TPL-0005, TPL-0013).
package notification

import (
	"errors"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/adapter/ses"
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
	// SMTPAddr is the host and port of an SMTP server, such as
	// mailpit:1025. When it is set, Notification speaks plain SMTP to it,
	// without authentication or TLS, as the local environment does. When it
	// is empty, Notification sends its mail through Amazon SES, as it does in
	// AWS.
	SMTPAddr string
	// AWS is the configuration of the AWS SDK that awsconfig.Load returns.
	// Amazon SES is called with its region, credentials, and endpoint.
	AWS aws.Config
	// MailFrom is the address that the mail comes from, such as
	// "Meeting rooms <rooms@example.com>".
	MailFrom string
}

// NewHandler returns the handler of the events that Notification receives.
// It fails when the settings of the mail are not complete.
func NewHandler(deps Deps) (event.Handler, error) {
	mailer, err := newMailer(deps)
	if err != nil {
		return nil, err
	}
	return app.NewNotifier(deps.Pool, mailer, deps.Now, deps.Logger).HandleEvent, nil
}

// newMailer chooses the mailer: SMTP when deps names an SMTP server, and
// Amazon SES when it does not. It logs which one sends the mail.
func newMailer(deps Deps) (app.Mailer, error) {
	if deps.SMTPAddr != "" {
		mailer, err := smtp.New(smtp.Config{Addr: deps.SMTPAddr, From: deps.MailFrom, Now: deps.Now})
		if err != nil {
			return nil, err
		}
		deps.Logger.Info("the mail goes out over SMTP",
			slog.String("mailer", "smtp"), slog.String("smtp_addr", deps.SMTPAddr))
		return mailer, nil
	}
	if deps.AWS.Region == "" {
		return nil, errors.New("notification: no SMTP server is set, and Amazon SES needs the region of the AWS configuration")
	}
	mailer, err := ses.New(ses.Config{Client: sesv2.NewFromConfig(deps.AWS), From: deps.MailFrom})
	if err != nil {
		return nil, err
	}
	deps.Logger.Info("the mail goes out through Amazon SES",
		slog.String("mailer", "ses"), slog.String("region", deps.AWS.Region))
	return mailer, nil
}
