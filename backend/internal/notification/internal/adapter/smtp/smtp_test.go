package smtp_test

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/adapter/smtp"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
)

var now = time.Date(2026, 11, 18, 9, 30, 15, 0, time.UTC)

// envelope is what one SMTP session delivered: the sender and the recipients
// of the envelope, and the message, with "\n" at the end of each line.
type envelope struct {
	from string
	to   []string
	data string
}

// server is a small SMTP server for the tests. It answers every command with
// success, except the command in refuse, and keeps what each session
// delivered. With silent, it accepts connections and never answers.
type server struct {
	addr     string
	refuse   string // MAIL, RCPT, DATA, or "." for the end of the data
	answer   string // the answer to the refused command, such as "550 5.1.1 no such user"
	silent   bool
	mu       sync.Mutex
	conns    int
	received []envelope
}

func startServer(t *testing.T, configure func(*server)) *server {
	t.Helper()
	s := &server{}
	if configure != nil {
		configure(s)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.addr = ln.Addr().String()
	var sessions sync.WaitGroup
	t.Cleanup(func() {
		_ = ln.Close()
		sessions.Wait()
	})
	sessions.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.conns++
			s.mu.Unlock()
			sessions.Go(func() { s.serve(t, conn) })
		}
	})
	return s
}

func (s *server) serve(t *testing.T, conn net.Conn) {
	t.Helper()
	defer func() { _ = conn.Close() }()
	// No session outlives the test.
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if s.silent {
		_, _ = io.Copy(io.Discard, conn)
		return
	}
	c := textproto.NewConn(conn)
	reply := func(line string) bool { return c.PrintfLine("%s", line) == nil }
	if !reply("220 test.example.com ESMTP") {
		return
	}
	var current envelope
	for {
		line, err := c.ReadLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		verb = strings.ToUpper(verb)
		if s.refuse == verb {
			reply(s.answer)
			continue
		}
		switch verb {
		case "EHLO":
			// The client of net/smtp greets with EHLO, and falls back to the
			// older command only when EHLO fails.
			reply("250 test.example.com")
		case "MAIL":
			current = envelope{from: path(arg, "FROM:")}
			reply("250 2.1.0 OK")
		case "RCPT":
			current.to = append(current.to, path(arg, "TO:"))
			reply("250 2.1.5 OK")
		case "DATA":
			reply("354 End data with <CR><LF>.<CR><LF>")
			data, err := c.ReadDotBytes()
			if err != nil {
				t.Errorf("the server could not read the data: %v", err)
				return
			}
			if s.refuse == "." {
				reply(s.answer)
				continue
			}
			current.data = string(data)
			s.mu.Lock()
			s.received = append(s.received, current)
			s.mu.Unlock()
			reply("250 2.0.0 OK queued")
		case "QUIT":
			reply("221 2.0.0 Bye")
			return
		default:
			reply("502 5.5.2 command not recognized")
		}
	}
}

// path returns the address in an argument such as "FROM:<a@example.com>".
func path(arg, prefix string) string {
	if len(arg) < len(prefix) || !strings.EqualFold(arg[:len(prefix)], prefix) {
		return "?" + arg
	}
	address, _, _ := strings.Cut(arg[len(prefix):], " ")
	return strings.Trim(address, "<>")
}

func (s *server) delivered() []envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]envelope(nil), s.received...)
}

func (s *server) connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns
}

func newMailer(t *testing.T, addr string, configure func(*smtp.Config)) *smtp.Mailer {
	t.Helper()
	c := smtp.Config{Addr: addr, From: "Meeting rooms <rooms@example.com>", Now: func() time.Time { return now }}
	if configure != nil {
		configure(&c)
	}
	m, err := smtp.New(c)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func message() app.Message {
	return app.Message{
		To:      "organizer@example.com",
		Subject: "Reservation confirmed: Fri, 20 Nov 2026 01:00 UTC",
		Body: "Your reservation of a meeting room is confirmed.\n\n" +
			"Reservation: 0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a\n" +
			".a line that starts with a dot\n" +
			"a=b, and a line long enough to be folded by quoted-printable, which allows 76 characters on a line\n",
	}
}

// read parses what the server received as an RFC 5322 message, and decodes
// its body.
func read(t *testing.T, data string) (*mail.Message, string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(data))
	if err != nil {
		t.Fatalf("the server received no RFC 5322 message: %v\n%s", err, data)
	}
	body, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatalf("the body is not quoted-printable: %v", err)
	}
	return msg, strings.ReplaceAll(string(body), "\r\n", "\n")
}

func TestSendDeliversTheMessage(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	mailer := newMailer(t, s.addr, nil)
	want := message()

	if err := mailer.Send(t.Context(), want); err != nil {
		t.Fatal(err)
	}
	got := s.delivered()
	if len(got) != 1 {
		t.Fatalf("the server received %d messages, want 1", len(got))
	}
	if got[0].from != "rooms@example.com" {
		t.Errorf("the sender of the envelope is %q, want rooms@example.com", got[0].from)
	}
	if len(got[0].to) != 1 || got[0].to[0] != "organizer@example.com" {
		t.Errorf("the recipients of the envelope are %q, want [organizer@example.com]", got[0].to)
	}

	msg, body := read(t, got[0].data)
	headers := map[string]string{
		"From":                      `"Meeting rooms" <rooms@example.com>`,
		"To":                        "<organizer@example.com>",
		"Subject":                   want.Subject,
		"Mime-Version":              "1.0",
		"Content-Type":              "text/plain; charset=utf-8",
		"Content-Transfer-Encoding": "quoted-printable",
	}
	for name, value := range headers {
		if got := msg.Header.Get(name); got != value {
			t.Errorf("the header %s is %q, want %q", name, got, value)
		}
	}
	if date, err := msg.Header.Date(); err != nil || !date.Equal(now) {
		t.Errorf("the header Date is %q (%v), want %s", msg.Header.Get("Date"), err, now)
	}
	if id := msg.Header.Get("Message-Id"); !regexp.MustCompile(`^<[0-9a-f-]{36}@example\.com>$`).MatchString(id) {
		t.Errorf("the header Message-ID is %q, want <uuid@example.com>", id)
	}
	if body != want.Body {
		t.Errorf("the body is\n%s\nwant\n%s", body, want.Body)
	}
}

func TestEachMessageHasItsOwnID(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	mailer := newMailer(t, s.addr, nil)
	for range 2 {
		if err := mailer.Send(t.Context(), message()); err != nil {
			t.Fatal(err)
		}
	}
	got := s.delivered()
	if len(got) != 2 {
		t.Fatalf("the server received %d messages, want 2", len(got))
	}
	first, _ := read(t, got[0].data)
	second, _ := read(t, got[1].data)
	if first.Header.Get("Message-Id") == second.Header.Get("Message-Id") {
		t.Errorf("two messages have the same Message-ID %s", first.Header.Get("Message-Id"))
	}
}

func TestASubjectThatIsNotASCIIIsEncoded(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	mailer := newMailer(t, s.addr, nil)
	want := message()
	want.Subject = "会議室の予約が確定しました"
	want.Body = "予約が確定しました。\n"

	if err := mailer.Send(t.Context(), want); err != nil {
		t.Fatal(err)
	}
	got := s.delivered()
	if len(got) != 1 {
		t.Fatalf("the server received %d messages, want 1", len(got))
	}
	for i := range len(got[0].data) {
		if got[0].data[i] > 0x7f {
			t.Fatalf("the message holds a byte that is not ASCII at %d:\n%s", i, got[0].data)
		}
	}
	msg, body := read(t, got[0].data)
	raw := msg.Header.Get("Subject")
	if !strings.HasPrefix(raw, "=?utf-8?q?") {
		t.Errorf("the subject %q is not an RFC 2047 encoded word", raw)
	}
	var decoder mime.WordDecoder
	if subject, err := decoder.DecodeHeader(raw); err != nil || subject != want.Subject {
		t.Errorf("the subject decodes to %q (%v), want %q", subject, err, want.Subject)
	}
	if body != want.Body {
		t.Errorf("the body is %q, want %q", body, want.Body)
	}
}

// A line break in an address or in the subject would let the text after it
// pass as another header, such as a Bcc. Send refuses it and connects to no
// server.
func TestSendRefusesALineBreakInAHeader(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	mailer := newMailer(t, s.addr, nil)
	tests := map[string]func(*app.Message){
		"CR LF in the subject":   func(m *app.Message) { m.Subject = "Hello\r\nBcc: victim@example.com" },
		"LF in the subject":      func(m *app.Message) { m.Subject = "Hello\nBcc: victim@example.com" },
		"CR in the subject":      func(m *app.Message) { m.Subject = "Hello\rBcc: victim@example.com" },
		"CR LF in the recipient": func(m *app.Message) { m.To = "organizer@example.com\r\nBcc: victim@example.com" },
		"LF in the recipient":    func(m *app.Message) { m.To = "organizer@example.com\nBcc: victim@example.com" },
		"two recipients":         func(m *app.Message) { m.To = "organizer@example.com, victim@example.com" },
	}
	for name, change := range tests {
		msg := message()
		change(&msg)
		if err := mailer.Send(t.Context(), msg); err == nil {
			t.Errorf("%s: Send = nil, want an error", name)
		}
	}
	if n := s.connections(); n != 0 {
		t.Errorf("the mailer connected %d times, want never", n)
	}
}

func TestNewRefusesAnIncompleteConfig(t *testing.T) {
	t.Parallel()
	tests := map[string]smtp.Config{
		"no address":            {From: "rooms@example.com"},
		"an address no port":    {Addr: "mailpit", From: "rooms@example.com"},
		"no host":               {Addr: ":1025", From: "rooms@example.com"},
		"no sender":             {Addr: "mailpit:1025"},
		"a sender no address":   {Addr: "mailpit:1025", From: "rooms"},
		"a line break in From":  {Addr: "mailpit:1025", From: "rooms@example.com\r\nBcc: victim@example.com"},
		"two senders":           {Addr: "mailpit:1025", From: "a@example.com, b@example.com"},
		"a sender with a break": {Addr: "mailpit:1025", From: "Rooms\n <rooms@example.com>"},
	}
	for name, c := range tests {
		if _, err := smtp.New(c); err == nil {
			t.Errorf("%s: New = nil error, want one", name)
		}
	}
}

func TestSendFailsWhenTheServerRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		refuse, answer, want string
	}{
		{"MAIL", "451 4.3.0 try again later", "MAIL FROM"},
		{"RCPT", "550 5.1.1 no such user", "RCPT TO"},
		{"DATA", "554 5.5.1 no valid recipients", "DATA"},
		{".", "552 5.3.4 message too big", "DATA"},
	}
	for _, tt := range tests {
		t.Run(tt.refuse, func(t *testing.T) {
			t.Parallel()
			s := startServer(t, func(s *server) { s.refuse, s.answer = tt.refuse, tt.answer })
			mailer := newMailer(t, s.addr, nil)
			err := mailer.Send(t.Context(), message())
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), tt.answer[:3]) {
				t.Errorf("Send = %v, want an error that names %s and the code %s", err, tt.want, tt.answer[:3])
			}
			var protocol *textproto.Error
			if !errors.As(err, &protocol) {
				t.Errorf("Send = %v, want the answer of the server in the error", err)
			}
			if got := s.delivered(); len(got) != 0 {
				t.Errorf("the server kept %d messages, want none", len(got))
			}
		})
	}
}

func TestSendFailsWithoutAServer(t *testing.T) {
	t.Parallel()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	if err := newMailer(t, addr, nil).Send(t.Context(), message()); err == nil {
		t.Error("Send = nil with no server listening, want an error")
	}
}

// A server that never answers holds Send no longer than the deadline of its
// context, or the timeout of the mailer.
func TestSendGivesUpOnASilentServer(t *testing.T) {
	t.Parallel()
	s := startServer(t, func(s *server) { s.silent = true })
	const wait = 200 * time.Millisecond
	tests := []struct {
		name   string
		mailer *smtp.Mailer
		// deadline and cancel end the context of Send after that long,
		// when they are not zero.
		deadline, cancel time.Duration
		want             error
	}{
		{
			name:     "the deadline of the context",
			mailer:   newMailer(t, s.addr, nil),
			deadline: wait,
			want:     os.ErrDeadlineExceeded,
		},
		{
			name:   "the cancellation of the context",
			mailer: newMailer(t, s.addr, nil),
			cancel: wait,
			want:   context.Canceled,
		},
		{
			name:   "the timeout of the mailer",
			mailer: newMailer(t, s.addr, func(c *smtp.Config) { c.Timeout = wait }),
			want:   os.ErrDeadlineExceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			if tt.deadline > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.deadline)
				defer cancel()
			}
			if tt.cancel > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				time.AfterFunc(tt.cancel, cancel)
			}
			start := time.Now()
			err := tt.mailer.Send(ctx, message())
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Errorf("Send took %s, want about 200ms", elapsed)
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("Send = %v, want %v", err, tt.want)
			}
		})
	}
}
