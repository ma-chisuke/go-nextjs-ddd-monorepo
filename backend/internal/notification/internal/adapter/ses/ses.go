// Package ses sends the mail of the Notification context through Amazon SES,
// with the operation SendEmail of the SES API v2. The worker uses it in AWS,
// where its task role may call ses:SendEmail; the local environment uses the
// adapter smtp and Mailpit instead (TPL-0017).
//
// The mail is the one that the adapter smtp writes: the same sender,
// recipient, subject, and body, in plain text. It goes to SES as Simple
// content, in UTF-8, and SES writes the message: the headers, the Message-ID
// and the Date, and the encoding of the subject and the body.
//
// The client is made from the configuration of package awsconfig, so the
// endpoint and the CA of AWS Emulator apply locally as they do for EventBridge
// and SQS, and in AWS the SDK reaches SES under its default name.
//
// # Failures
//
// Send returns nil only when SES took the mail. Every failure is an error, as
// in the adapter smtp, and the error keeps the answer of SES, which errors.As
// finds as a smithy.APIError with its code. The handler of Notification then
// rolls the record of the mail back, and the event comes again (TPL-0013):
//
//   - A throttling (TooManyRequestsException, LimitExceededException), an
//     answer 5xx, and a broken connection are tried again by the SDK, up to
//     three attempts within one handling, as for every AWS client of the
//     template. When they keep failing, the event comes again later.
//   - A refusal, an answer 4xx such as MessageRejected (a sender that no
//     verified identity covers, or, in the sandbox of SES, a recipient that is
//     not verified), MailFromDomainNotVerifiedException,
//     SendingPausedException, or AccountSuspendedException, fails the same
//     way, and the SDK does not try it again. SES refuses the mail again at
//     each delivery, and after the last one the queue moves the event to its
//     dead-letter queue. The same happens to a permanent refusal (5xx) of an
//     SMTP server, and to an event whose data breaks its contract. Most
//     refusals end when a person fixes the account or the identity, and the
//     events can then be moved back from the dead-letter queue. Taking a
//     refused mail as handled would record a mail that was never sent, and
//     lose its event.
//
// SendEmail takes no key that would let SES recognize a mail that comes
// again. When SES took a mail and its answer is lost, the next attempt sends
// it a second time: the window that the README of Notification describes.
package ses

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/notification/internal/app"
)

// DefaultTimeout bounds a Send whose context ends later, or never.
const DefaultTimeout = 30 * time.Second

// charset is the character set of the subject and the body.
const charset = "UTF-8"

// Client is the operation of the SES API v2 that the Mailer calls.
// *sesv2.Client implements it.
type Client interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

var _ Client = (*sesv2.Client)(nil)

// Config says how the Mailer reaches SES.
type Config struct {
	// Client calls SES: sesv2.NewFromConfig with the configuration that
	// awsconfig.Load returns.
	Client Client
	// From is the address that the mail comes from. It may carry a name, as
	// in "Meeting rooms <rooms@example.com>". SES sends only from an address
	// that a verified identity covers, such as the domain example.com.
	From string
	// Timeout bounds each Send, unless its context ends earlier. Default:
	// DefaultTimeout.
	Timeout time.Duration
}

// Mailer sends each message with one call of SendEmail. It implements
// app.Mailer.
type Mailer struct {
	client  Client
	from    string
	timeout time.Duration
}

var _ app.Mailer = (*Mailer)(nil)

// New returns the Mailer of c, or an error when c is not complete.
func New(c Config) (*Mailer, error) {
	if c.Client == nil {
		return nil, errors.New("ses: no client of SES")
	}
	from, err := parseAddress(c.From)
	if err != nil {
		return nil, fmt.Errorf("ses: the sender: %w", err)
	}
	m := &Mailer{client: c.Client, from: format(from), timeout: c.Timeout}
	if m.timeout <= 0 {
		m.timeout = DefaultTimeout
	}
	return m, nil
}

// parseAddress reads one email address, as the adapter smtp does. A line
// break is refused before anything else: in a header, the text after it would
// pass as another header (header injection). SES takes addresses in ASCII
// only; a domain that is not ASCII would have to be in Punycode.
func parseAddress(s string) (*mail.Address, error) {
	if strings.ContainsAny(s, "\r\n") {
		return nil, fmt.Errorf("the address %q holds a line break", s)
	}
	a, err := mail.ParseAddress(s)
	if err != nil {
		return nil, fmt.Errorf("%q is not an email address: %w", s, err)
	}
	for _, r := range a.Address {
		if r > 0x7f {
			return nil, fmt.Errorf("the address %q is not ASCII, and SES takes no other", a.Address)
		}
	}
	return a, nil
}

// format writes an address as SES takes it: the address alone, or with its
// name, which is encoded as RFC 2047 says when it is not ASCII.
func format(a *mail.Address) string {
	if a.Name == "" {
		return a.Address
	}
	return a.String()
}

// Send hands msg to SES, and returns nil only when SES took it. It refuses a
// recipient or a subject with a line break before it calls SES. It gives up
// when ctx ends or the timeout passes, whichever comes first.
func (m *Mailer) Send(ctx context.Context, msg app.Message) error {
	to, err := parseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("ses: the recipient: %w", err)
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return fmt.Errorf("ses: the subject %q holds a line break", msg.Subject)
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	if _, err := m.client.SendEmail(ctx, m.input(to, msg)); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("ses: %w", errors.Join(ctx.Err(), err))
		}
		return fmt.Errorf("ses: %w", err)
	}
	return nil
}

// input is the request of SendEmail for msg: one recipient, and Simple
// content in plain text.
func (m *Mailer) input(to *mail.Address, msg app.Message) *sesv2.SendEmailInput {
	return &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(m.from),
		Destination:      &types.Destination{ToAddresses: []string{format(to)}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: text(msg.Subject),
				Body:    &types.Body{Text: text(msg.Body)},
			},
		},
	}
}

func text(s string) *types.Content {
	return &types.Content{Data: aws.String(s), Charset: aws.String(charset)}
}
