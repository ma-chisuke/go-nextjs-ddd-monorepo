package app

import "context"

// Message is a mail to one person, in plain text.
type Message struct {
	// To is the email address of the recipient.
	To string
	// Subject is one line, without a line break.
	Subject string
	// Body is the text of the mail. Lines end in "\n".
	Body string
}

// Mailer sends mail. The SMTP adapter implements it for Mailpit, and
// apptest.Mailer in memory for tests.
type Mailer interface {
	// Send hands the message to the mail server, and returns nil only when
	// the server took it. It gives up when ctx ends.
	Send(ctx context.Context, m Message) error
}
