package notification

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

// smtpDeps are the settings of the local environment: an SMTP server.
func smtpDeps() Deps {
	return Deps{
		Logger:   slog.New(slog.DiscardHandler),
		Now:      time.Now,
		SMTPAddr: "mailpit:1025",
		MailFrom: "Meeting rooms <rooms@example.com>",
	}
}

// sesDeps are the settings in AWS: no SMTP server, and the configuration of
// the AWS SDK. Nothing in these tests calls SES.
func sesDeps() Deps {
	return Deps{
		Logger: slog.New(slog.DiscardHandler),
		Now:    time.Now,
		AWS: aws.Config{
			Region:      "ap-northeast-1",
			Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
		},
		MailFrom: "Meeting rooms <rooms@example.com>",
	}
}

func TestNewHandlerRefusesIncompleteMailSettings(t *testing.T) {
	t.Parallel()
	smtpNoSender, sesNoSender, noRegion := smtpDeps(), sesDeps(), sesDeps()
	smtpNoSender.MailFrom = ""
	sesNoSender.MailFrom = ""
	noRegion.AWS.Region = ""
	tests := map[string]Deps{
		"SMTP without a sender":                  smtpNoSender,
		"SES without a sender":                   sesNoSender,
		"neither an SMTP server nor a region":    noRegion,
		"an SMTP server that is not a host:port": func() Deps { d := smtpDeps(); d.SMTPAddr = "mailpit"; return d }(),
	}
	for name, d := range tests {
		if _, err := NewHandler(d); err == nil {
			t.Errorf("%s: NewHandler = nil error, want one", name)
		}
	}
}

// The worker sends over SMTP when it is given an SMTP server, and through
// SES when it is not, and says which in one line of its log.
func TestNewHandlerChoosesTheMailer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		deps Deps
		want map[string]string
	}{
		{"an SMTP server", smtpDeps(), map[string]string{"mailer": "smtp", "smtp_addr": "mailpit:1025"}},
		{"no SMTP server", sesDeps(), map[string]string{"mailer": "ses", "region": "ap-northeast-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var log bytes.Buffer
			tt.deps.Logger = slog.New(slog.NewJSONHandler(&log, nil))
			if _, err := NewHandler(tt.deps); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(log.String()), "\n")
			if len(lines) != 1 {
				t.Fatalf("the log has %d lines, want one:\n%s", len(lines), log.String())
			}
			var record map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
				t.Fatal(err)
			}
			for key, value := range tt.want {
				if record[key] != value {
					t.Errorf("the log says %s=%v, want %s:\n%s", key, record[key], value, lines[0])
				}
			}
		})
	}
}

// The handler that NewHandler wires handles events: one of a type that
// Notification does not handle is ignored before the database or the mail
// server is needed.
func TestTheHandlerHandlesEvents(t *testing.T) {
	t.Parallel()
	for name, deps := range map[string]Deps{"SMTP": smtpDeps(), "SES": sesDeps()} {
		handler, err := NewHandler(deps)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		e, err := event.New(uuid.Must(uuid.NewV7()), event.Source("facility"), "facility.RoomRegistered.v1", "",
			time.Now(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if err := handler(t.Context(), e); err != nil {
			t.Errorf("%s: the handler = %v, want nil", name, err)
		}
	}
}
