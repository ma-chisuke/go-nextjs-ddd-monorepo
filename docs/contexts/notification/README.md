# Notification

Notification is responsible for telling the organizer of a meeting about their reservation: it sends a mail when a reservation is confirmed and when it is cancelled.
It is not responsible for the reservations themselves: Reservation is, and Notification knows of them only what the events of Reservation carry.
The words below are those of the [glossary](glossary.md).

## Kind of subdomain

Supporting. Sending a mail when something happened is needed, but it does not set the product apart, and Notification has one rule of its own: each event sends its mail once.
So Notification is a transaction script (TPL-0003): it has no `domain` package, and its use case in `internal/app/` reads the event, calls the queries that sqlc generates, and hands the mail to a mailer.
It is not a generic subdomain, because the texts and the choice of the events to tell are its own; only the delivery of the mail is generic, and the mail server does that.

## Public API

None. Notification is a worker: it receives the events of Reservation from a queue of its own, and answers no request, so it has no API document and no problems.
Other contexts reach it only by publishing events (TPL-0005, TPL-0013).

## Events

Notification publishes no event. It consumes these events of Reservation, as [`contracts/events/notification.asyncapi.yaml`](../../../contracts/events/notification.asyncapi.yaml) declares:

| Event | Contract of its data | What Notification does |
|---|---|---|
| `reservation.ReservationConfirmed.v1` | [`ReservationConfirmed.v1.json`](../../../contracts/events/reservation/ReservationConfirmed.v1.json) | Sends the organizer a mail that the reservation is confirmed, with its period and attendees. Reservation publishes this event again when a reservation is rescheduled, so a rescheduling sends another such mail, with the new period. |
| `reservation.ReservationCancelled.v1` | [`ReservationCancelled.v1.json`](../../../contracts/events/reservation/ReservationCancelled.v1.json) | Sends the organizer a mail that the reservation is cancelled. |

The worker is the command `notification` (TPL-0014). It reads the events from the SQS queue that `EVENTS_QUEUE_URL` names, `notification-events`, into which a rule of Notification selects these two types from the event bus. The module `infra/modules/event-subscription` creates the queue, its dead-letter queue `notification-events-dlq`, and the rule; locally, `infra/local/main.tf` calls it.

An event of any other type is logged and ignored, and counts as handled.
An event of one of these types whose data breaks its contract, such as one without the address of the organizer, fails: it comes again, and after the last attempt the broker sets it aside in the dead-letter queue of Notification.

## Rules

- Each event sends its mail once, although delivery is at least once (TPL-0013). In one transaction, Notification records the mail in `notifications`, whose unique constraint on the id of the event refuses a second record, then sends the mail, and commits. When the record is there already, it sends nothing.
- Two deliveries of one event at the same moment send one mail: the second record waits for the transaction of the first, and finds the record after its commit.
- When the mail fails, the record rolls back, and the event comes again later.
- One case sends a mail twice: the mail is sent, and the commit of its record then fails. The event comes again and finds no record. Delivery at least once cannot avoid this without the help of the mail server, which would have to recognize the mail when it comes again. Notification keeps the window short: once the mail is sent, the commit no longer stops at the deadline of the delivery.
- Events can arrive out of order, for example the cancellation of a reservation before its confirmation. Notification sends each mail as it comes and does not compare the events of one reservation, so the organizer can get the mails in the order in which they arrived.

## Known limits of the sample

- A mail shows the room by its id and the period in UTC, because the events carry neither the name of the room nor its time zone. A product would add them to the events, in a new version of each, or ask Facility for each mail.
- The mailer speaks plain SMTP, without authentication or TLS, to Mailpit in the local environment. In AWS, the mail will go through Amazon SES, with another adapter of `Mailer` that arrives with the infrastructure milestone.

## What Notification needs from other contexts

- From Reservation, its events, as their JSON Schemas define them. `internal/app/events.go` is the anticorruption layer: it reads the data of each event into structs of Notification and checks the properties that the mail needs. It ignores the properties that it does not read, so Reservation can add properties without breaking Notification. The unit tests turn every example in the two JSON Schemas into a mail, so an example that Notification cannot read fails them.
- Nothing from Facility.
- A mail server that accepts SMTP. The command gives its address and the address that the mail comes from.
