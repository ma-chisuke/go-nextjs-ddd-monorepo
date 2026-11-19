// Command notification runs the worker of the Notification context: it reads
// the events of the queue of the context and sends mail to the organizer.
// Notification has no HTTP API (TPL-0013, TPL-0014).
//
// Configuration, from the environment:
//
//	DATABASE_URL      the PostgreSQL URL of the runtime role of Notification (required)
//	EVENTS_QUEUE_URL  the URL of the SQS queue that holds the events for Notification (required)
//	MAIL_FROM         the address that the mail comes from, such as "Meeting rooms <rooms@example.com>" (required)
//	SMTP_ADDR         the host and port of an SMTP server, such as mailpit:1025 (default: none, and the mail
//	                  goes through Amazon SES)
//	AWS_REGION        the AWS region, of SQS and of SES (required; ECS sets it)
//	HTTP_ADDR         the address of the health check (default ":8080")
//	LOG_LEVEL         debug, info, warn, or error (default "info")
//
// The worker sends its mail over SMTP when SMTP_ADDR is set, as the local
// environment does to Mailpit, and through Amazon SES when it is not, as in
// AWS. Its log says at start which of the two it uses. With SES, a verified
// identity of SES must cover MAIL_FROM, and the role of the task must allow
// ses:SendEmail on that identity.
//
// The local environment also points the worker at AWS Emulator, with the
// variables that package awsconfig describes.
//
// With the argument healthcheck, it asks the worker that runs on this host
// for its health (GET /healthz on HTTP_ADDR) and exits with 0 when the worker
// is healthy and 1 when it is not. The container image, which has no shell,
// runs it as its health check.
package main

import (
	"context"
	"io"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsconfig"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsevents"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/config"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/logging"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres"
)

func main() {
	httpserver.Main("notification", run)
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	env := config.NewReader(getenv)
	databaseURL := env.Required("DATABASE_URL")
	queueURL := env.Required("EVENTS_QUEUE_URL")
	mailFrom := env.Required("MAIL_FROM")
	smtpAddr := env.Default("SMTP_ADDR", "")
	addr := env.Default(httpserver.AddrVariable, httpserver.DefaultAddr)
	logLevel := env.Default("LOG_LEVEL", "info")
	if err := env.Err(); err != nil {
		return err
	}
	logger, err := logging.New(stdout, logLevel, "notification")
	if err != nil {
		return err
	}
	cfg, err := awsconfig.Load(ctx, getenv)
	if err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	handle, err := notification.NewHandler(notification.Deps{
		Pool:     pool,
		Logger:   logger,
		Now:      time.Now,
		SMTPAddr: smtpAddr,
		AWS:      cfg,
		MailFrom: mailFrom,
	})
	if err != nil {
		return err
	}
	consumer := awsevents.NewConsumer(cfg, queueURL, logger, awsevents.ConsumerConfig{})
	return httpserver.RunWorker(ctx, addr, logger, func(ctx context.Context) error {
		return consumer.Run(ctx, handle)
	})
}
