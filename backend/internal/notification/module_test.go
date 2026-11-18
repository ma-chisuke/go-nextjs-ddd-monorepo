package notification

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

func deps() Deps {
	return Deps{
		Logger:   slog.New(slog.DiscardHandler),
		Now:      time.Now,
		SMTPAddr: "mailpit:1025",
		MailFrom: "Meeting rooms <rooms@example.com>",
	}
}

func TestNewHandlerRefusesIncompleteMailSettings(t *testing.T) {
	t.Parallel()
	noServer, noSender := deps(), deps()
	noServer.SMTPAddr = ""
	noSender.MailFrom = ""
	for name, d := range map[string]Deps{"no mail server": noServer, "no sender": noSender} {
		if _, err := NewHandler(d); err == nil {
			t.Errorf("%s: NewHandler = nil error, want one", name)
		}
	}
}

// The handler that NewHandler wires handles events: one of a type that
// Notification does not handle is ignored before the database or the mail
// server is needed.
func TestTheHandlerHandlesEvents(t *testing.T) {
	t.Parallel()
	handler, err := NewHandler(deps())
	if err != nil {
		t.Fatal(err)
	}
	e, err := event.New(uuid.Must(uuid.NewV7()), event.Source("facility"), "facility.RoomRegistered.v1", "",
		time.Now(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := handler(t.Context(), e); err != nil {
		t.Errorf("the handler = %v, want nil", err)
	}
}
