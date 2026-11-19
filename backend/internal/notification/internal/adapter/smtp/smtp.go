// Package smtp sends the mail of the Notification context over SMTP, with the
// standard library. It speaks plain SMTP, without authentication or TLS, as
// the local mail server Mailpit expects (TPL-0017). In AWS, the adapter ses
// sends the mail through Amazon SES instead.
//
// Send returns nil only when the server took the mail. Every failure is an
// error that keeps the answer of the server, a permanent refusal (5xx) too:
// the handler of Notification then rolls the record of the mail back, the
// event comes again, and after the last delivery it waits in the dead-letter
// queue (TPL-0013). The adapter ses does the same with the answers of SES.
package smtp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	netsmtp "net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
)

// DefaultTimeout bounds a Send whose context ends later, or never.
const DefaultTimeout = 30 * time.Second

// Config says how the Mailer reaches its mail server.
type Config struct {
	// Addr is the host and port of the mail server, such as mailpit:1025.
	Addr string
	// From is the address that the mail comes from: the sender of the SMTP
	// envelope, and the From header. It may carry a name, as in
	// "Meeting rooms <rooms@example.com>".
	From string
	// Timeout bounds each Send, unless its context ends earlier. Default:
	// DefaultTimeout.
	Timeout time.Duration
	// Now gives the current time, for the Date header. Default: time.Now.
	Now func() time.Time
}

// Mailer sends each message over a new SMTP connection. It implements
// app.Mailer.
type Mailer struct {
	addr    string
	host    string
	from    *mail.Address
	domain  string
	timeout time.Duration
	now     func() time.Time
}

var _ app.Mailer = (*Mailer)(nil)

// New returns the Mailer of c, or an error when c is not complete.
func New(c Config) (*Mailer, error) {
	host, _, err := net.SplitHostPort(c.Addr)
	if err != nil || host == "" {
		return nil, fmt.Errorf("smtp: the address of the mail server %q is not host:port", c.Addr)
	}
	from, err := parseAddress(c.From)
	if err != nil {
		return nil, fmt.Errorf("smtp: the sender: %w", err)
	}
	m := &Mailer{
		addr:    c.Addr,
		host:    host,
		from:    from,
		domain:  from.Address[strings.LastIndex(from.Address, "@")+1:],
		timeout: c.Timeout,
		now:     c.Now,
	}
	if m.timeout <= 0 {
		m.timeout = DefaultTimeout
	}
	if m.now == nil {
		m.now = time.Now
	}
	return m, nil
}

// parseAddress reads one email address. A line break is refused before
// anything else: in a header or an SMTP command, the text after it would
// pass as another header or command (header injection).
func parseAddress(s string) (*mail.Address, error) {
	if strings.ContainsAny(s, "\r\n") {
		return nil, fmt.Errorf("the address %q holds a line break", s)
	}
	a, err := mail.ParseAddress(s)
	if err != nil {
		return nil, fmt.Errorf("%q is not an email address: %w", s, err)
	}
	return a, nil
}

// Send hands msg to the mail server, and returns nil only when the server
// took it. It refuses a recipient or a subject with a line break before it
// connects. It gives up when ctx ends or the timeout passes, whichever comes
// first.
func (m *Mailer) Send(ctx context.Context, msg app.Message) error {
	to, err := parseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("smtp: the recipient: %w", err)
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return fmt.Errorf("smtp: the subject %q holds a line break", msg.Subject)
	}
	data, err := m.message(to, msg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return fmt.Errorf("smtp: connect to %s: %w", m.addr, err)
	}
	// Closing a connection whose conversation ended changes nothing.
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	// When ctx ends during the conversation, the next read or write fails at
	// once, instead of at the deadline.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()

	if err := m.converse(conn, to.Address, data); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("smtp: %s: %w", m.addr, errors.Join(ctx.Err(), err))
		}
		return fmt.Errorf("smtp: %s: %w", m.addr, err)
	}
	return nil
}

// converse sends one message over conn: MAIL, RCPT, DATA, and QUIT.
func (m *Mailer) converse(conn net.Conn, to string, data []byte) error {
	c, err := netsmtp.NewClient(conn, m.host)
	if err != nil {
		return fmt.Errorf("greeting: %w", err)
	}
	// Close also closes conn, which Send closes again; that does no harm.
	defer func() { _ = c.Close() }()
	if err := c.Mail(m.from.Address); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	// Close ends the data, and returns the answer of the server to it.
	if err := w.Close(); err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	// The server took the message. A failed QUIT does not change that, and
	// an error here would make the event come again and the mail go twice.
	_ = c.Quit()
	return nil
}

// message writes msg as an RFC 5322 message in plain text, encoded as
// quoted-printable, so that it passes through any SMTP server. Every line
// ends in "\r\n", and the SMTP client escapes the dots at the start of lines.
func (m *Mailer) message(to *mail.Address, msg app.Message) ([]byte, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("smtp: the id of the message: %w", err)
	}
	var b bytes.Buffer
	header := func(name, value string) {
		b.WriteString(name + ": " + value + "\r\n")
	}
	header("Date", m.now().Format(time.RFC1123Z))
	header("From", m.from.String())
	header("To", to.String())
	// RFC 2047: a subject that is not printable ASCII is encoded, and one
	// that is stays as it is.
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("Message-ID", "<"+id.String()+"@"+m.domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	body := quotedprintable.NewWriter(&b)
	if _, err := body.Write([]byte(msg.Body)); err != nil {
		return nil, fmt.Errorf("smtp: the body: %w", err)
	}
	if err := body.Close(); err != nil {
		return nil, fmt.Errorf("smtp: the body: %w", err)
	}
	return b.Bytes(), nil
}
