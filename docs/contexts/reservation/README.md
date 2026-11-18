# Reservation

Reservation is responsible for the bookings of meeting rooms: it books a room for a period, reschedules a reservation, and cancels it, and it never lets two confirmed reservations of one room overlap.
It is not responsible for the rooms themselves: it asks Facility about a room each time it needs one, and keeps no copy. Telling the organizer about a booking belongs to Notification, which learns of it from the events of Reservation.
The words below are those of the [glossary](glossary.md).

## Kind of subdomain

Core. The booking rules are what sets the product apart, and they depend on each other: the time zone of a room decides on which day a period falls, that day decides the opening hours, and an overlap can arise between two requests that arrive at the same moment.
So Reservation has a domain model with ports and adapters (TPL-0003, TPL-0004).
The aggregate `Reservation` in `internal/domain/` applies the rules without I/O, and takes the current time and new ids as arguments. The use cases in `internal/app/` load what it needs through ports, and store the result.

## Public API

[`contracts/http/reservation.openapi.yaml`](../../../contracts/http/reservation.openapi.yaml):

| Operation | What it does | Problems of its own |
|---|---|---|
| `POST /reservations` (`bookRoom`) | Books a room, and answers 201 with the reservation, its URL in `Location`, and its version in `ETag`. It needs an `Idempotency-Key`: a request repeated with the same key and the same body gets the reservation that the first one made | The rules of a booking (below), [unknown-room](../../problems/unknown-room.md), [room-already-booked](../../problems/room-already-booked.md), [idempotency-key-reused](../../problems/idempotency-key-reused.md), [facility-unavailable](../../problems/facility-unavailable.md) |
| `GET /reservations` (`listReservations`) | Lists reservations, confirmed and cancelled, in the order of their start, a page at a time, for one room or for a window of time | [invalid-cursor](../../problems/invalid-cursor.md) |
| `GET /reservations/{reservationId}` (`getReservation`) | Returns one reservation, with its version in `ETag` | [reservation-not-found](../../problems/reservation-not-found.md) |
| `PATCH /reservations/{reservationId}` (`rescheduleReservation`) | Changes the period or the attendees of a confirmed reservation. It needs the version that the client read in `If-Match` | The rules of a booking, [reservation-cancelled](../../problems/reservation-cancelled.md), [reservation-not-found](../../problems/reservation-not-found.md), [unknown-room](../../problems/unknown-room.md), [room-already-booked](../../problems/room-already-booked.md), [facility-unavailable](../../problems/facility-unavailable.md), [precondition-required](../../problems/precondition-required.md), [precondition-failed](../../problems/precondition-failed.md) |
| `POST /reservations/{reservationId}/cancellation` (`cancelReservation`) | Cancels a confirmed reservation. It needs `If-Match` | [cancellation-too-late](../../problems/cancellation-too-late.md), [reservation-cancelled](../../problems/reservation-cancelled.md), [reservation-not-found](../../problems/reservation-not-found.md), [precondition-required](../../problems/precondition-required.md), [precondition-failed](../../problems/precondition-failed.md) |

The rules of a booking on the period answer with [period-out-of-order](../../problems/period-out-of-order.md), [period-misaligned](../../problems/period-misaligned.md), [period-too-long](../../problems/period-too-long.md), or [period-in-past](../../problems/period-in-past.md). Those on the attendees and the room answer with [no-attendees](../../problems/no-attendees.md), [over-capacity](../../problems/over-capacity.md), or [outside-opening-hours](../../problems/outside-opening-hours.md).
Every operation can also answer with the problems that every API shares: [malformed-request](../../problems/malformed-request.md), [validation-error](../../problems/validation-error.md), [unsupported-media-type](../../problems/unsupported-media-type.md), [not-found](../../problems/not-found.md), [method-not-allowed](../../problems/method-not-allowed.md), and [internal-error](../../problems/internal-error.md).
The API has no authentication of its own, because only the private network reaches it (TPL-0014).

## Events

Reservation publishes two integration events. [`contracts/events/reservation.asyncapi.yaml`](../../../contracts/events/reservation.asyncapi.yaml) describes them, and the JSON Schemas in [`contracts/events/reservation/`](../../../contracts/events/reservation/) define their data.

| Event | When Reservation publishes it |
|---|---|
| `reservation.ReservationConfirmed.v1` | When a room is booked, and again each time the reservation is rescheduled, with the new period and attendees |
| `reservation.ReservationCancelled.v1` | When a reservation is cancelled |

- Each change of the domain returns its domain event, `ReservationConfirmed` or `ReservationCancelled`. The store turns it into the integration event, a CloudEvents envelope whose `subject` is the id of the reservation, and writes it to the table `outbox` in the transaction of the change (TPL-0013). So an event is stored if and only if its change commits. A refused change stores no event, and neither does a booking that comes again with its idempotency key.
- The relay of the platform (`internal/platform/outbox`) reads the outbox in the order of the event ids, which is close to the order of the changes, hands each event to the broker, and then marks it as published. `NewRelay` in `module.go` builds it for a broker. Several relays can run at once, and each event goes to the one that takes it first.
- The relay runs as a process of its own, `reservation relay`: the command of Reservation with the argument `relay`, from the image of the API (TPL-0014). It publishes to the EventBridge bus that `EVENT_BUS_NAME` names, through `platform/awsevents`, and refuses to start when that bus does not exist.
- Delivery is at least once. When a relay stops after it handed an event to the broker and before it marked the event, the event is published again, with the same `id`. Events of one reservation can also arrive out of order. A consumer records the `id` of each event that it handled, and looks at `time` when the order matters.
- The relay deletes a published event from the outbox after a retention period, seven days by default.
- No event is dropped silently. A domain event that has no integration event fails the change. An event of the outbox that cannot be read stays there and is logged as an error, and the relay goes on with the other events.
- The tests check every integration event that the store makes against the JSON Schema of its type.

Reservation subscribes to no event.

## Rules

- A reservation books one room for a period, for a number of attendees, on behalf of an organizer. It is confirmed or cancelled.
- A period ends after it starts, starts and ends on a quarter hour, lasts at most 4 hours, and does not start in the past. It includes its start and excludes its end, so one reservation may end when the next one starts.
- At least 1 person attends, and the attendees fit the capacity of the room.
- The period lies inside the opening hours of the room on one day, read in the time zone of the room.
- Two confirmed reservations of one room never overlap. The use case checks first, and the exclusion constraint `reservations_no_overlap` decides between two requests that store at the same moment.
- A rescheduled reservation follows the same rules as a new one. A cancelled reservation is neither rescheduled nor cancelled again.
- A reservation can be cancelled until 30 minutes before it starts. Once it is cancelled, its room is free for its period.
- Each change names the version of the reservation that it read, and fails when the reservation changed since (optimistic locking).
- A booking that comes again with the same idempotency key and the same content books nothing new and gets the first reservation; with other content, it fails.

The numbers are constants of the domain: `PeriodStep`, `LongestPeriod`, and `CancellationNotice` in `internal/domain/period.go`.

## What Reservation needs from other contexts

For each booking and each rescheduling, Reservation needs the capacity, the opening hours, and the time zone of the room, which Facility owns.

- The port `RoomDirectory` in `internal/app/` says what Reservation needs, in its own words.
- The adapter `internal/adapter/facility/` implements it with `GET /rooms/{roomId}`, through the client that oapi-codegen generates from `contracts/http/facility.openapi.yaml`. It translates the room of Facility into `domain.Room`, so no type of Facility leaves the adapter (TPL-0005).
- Facility's problem room-not-found becomes [unknown-room](../../problems/unknown-room.md). A failure of Facility, an answer that cannot be used, or no answer within 3 seconds becomes [facility-unavailable](../../problems/facility-unavailable.md), with `Retry-After`.
- The command reads the base URL of Facility from `FACILITY_API_URL`.
- The tests of the adapter run against a fake of Facility that validates each request of the adapter, and each answer that the fake gives, against Facility's API document. A change to that document that the adapter does not follow fails them.

## Screens

The frontend books rooms with the feature `frontend/src/features/reservation/`, which calls this API from the server of Next.js only (TPL-0014). It knows a room in its own words (`room.ts`): the page gets the room from Facility, and `frontend/src/app/_compose/` turns it into those words, as `adapter/facility` does here.

| Page | What it shows | Operations |
|---|---|---|
| `/rooms/{roomId}` | The reservations of the room on one date in its time zone, with links to the dates around it, and the form that books the room on that date | `listReservations`, `bookRoom` |
| `/reservations/{reservationId}` | The reservation with its room, and the forms that reschedule and cancel it while it is confirmed | `getReservation`, `rescheduleReservation`, `cancelReservation` |

- A person reads and enters the period in the time zone of the room; `period.ts` turns it into the instants of the API, and says when the clocks of the zone skip a time.
- The form that books sends an `Idempotency-Key` that the server of Next.js makes when it renders the form, and keeps it with a fingerprint of the booking that went out with it. The same booking, sent again, books once; another booking from the same form gets a new key (`booking.ts`).
- The forms that reschedule and cancel send the version that the page showed in `If-Match`. On precondition-failed or reservation-cancelled, the page shows a notice and the reservation as it is now (`changes.ts`).
- Each kind of problem that a person can act on has words of its own, at the field that it is about where there is one (`problems.ts`). Any other problem shows its title and detail as they are.
