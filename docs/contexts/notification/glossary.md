# Glossary of Notification

The words of the Notification context, and the identifiers that stand for them in the code.
Notification has no domain layer (TPL-0003), so the identifiers belong to its `app` package, `backend/internal/notification/internal/app`.
`make check` fails when an exported type of that package has no row, or when a row names something that the package does not export (`lint-arch`).

| Identifier | Term | Meaning |
|---|---|---|
| `Notifier` | notifier | The use case of Notification. It turns each event of Reservation that it handles into one mail to the organizer, however often the event is delivered. |
| `Notifier.HandleEvent` | handle an event | Send the mail that an event asks for, unless the mail of that event was sent before, and record it. An event of a type that Notification does not handle is ignored. |
| `Message` | mail | A message in plain text to one person. |
| `Message.To` | recipient | The email address that a mail goes to: the organizer of the reservation. |
| `Message.Subject` | subject | The one line that names what a mail is about, such as the confirmation of a reservation and when it starts. |
| `Message.Body` | body | The text of a mail: what happened to the reservation, its id, the id of its room, and its period in UTC. |
| `Mailer` | mailer | What hands mail to a mail server. Locally it speaks SMTP to Mailpit, and in AWS it calls Amazon SES. |
| `Mailer.Send` | send a mail | Hand a mail to the mail server. A mail counts as sent only when the server took it. |
