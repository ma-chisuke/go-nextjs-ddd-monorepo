# Facility

Facility is responsible for the meeting rooms that people can book: their names, capacities, opening hours, and time zones.
It is not responsible for whether a room is free at a given time, or for who booked it: Reservation is.
The words below are those of the [glossary](glossary.md).

## Kind of subdomain

Supporting. A room is data that someone enters once and others read, under two rules, and nothing here sets the product apart from others.
So Facility is a transaction script (TPL-0003): it has no `domain` package, and its use cases in `internal/app/` check their input and call the queries that sqlc generates.
If its rules grow, for example opening hours that differ by weekday, the context moves to a domain model like Reservation's.

## Public API

[`contracts/http/facility.openapi.yaml`](../../../contracts/http/facility.openapi.yaml). Other contexts read rooms only through it, never through the database.

| Operation | What it does | Problems of its own |
|---|---|---|
| `POST /rooms` (`registerRoom`) | Registers a room, and answers 201 with its URL in `Location` | [room-name-taken](../../problems/room-name-taken.md), [invalid-opening-hours](../../problems/invalid-opening-hours.md), [unknown-time-zone](../../problems/unknown-time-zone.md) |
| `GET /rooms` (`listRooms`) | Lists the rooms in the order of their names, a page at a time | [invalid-cursor](../../problems/invalid-cursor.md) |
| `GET /rooms/{roomId}` (`getRoom`) | Returns one room | [room-not-found](../../problems/room-not-found.md) |

Every operation can also answer with the problems that every API shares: [malformed-request](../../problems/malformed-request.md), [validation-error](../../problems/validation-error.md), [unsupported-media-type](../../problems/unsupported-media-type.md), [not-found](../../problems/not-found.md), [method-not-allowed](../../problems/method-not-allowed.md), and [internal-error](../../problems/internal-error.md).
The API has no authentication of its own, because only the private network reaches it (TPL-0014).

## Events

Facility publishes no event and subscribes to none.

## Rules

- A room has a name that no other room has.
- Its capacity is a whole number from 1 to 1000.
- Its opening time is earlier than its closing time, so no room is open across midnight.
- Its time zone is an IANA time zone, in which its opening and closing times are read.

The database holds the first three rules as constraints of `rooms` as well. Only the use case checks the time zone.

## What Facility needs from other contexts

Nothing.

## Screens

The frontend shows rooms with the feature `frontend/src/features/facility/`, which calls this API from the server of Next.js only (TPL-0014).

| Page | What it shows | Operations |
|---|---|---|
| `/` | The rooms, in the order of their names, each with a link to its page | `listRooms`, one page of up to 100 rooms |
| `/rooms/new` | The form that registers a room. The time zone is a choice from the zones that the server of Next.js knows | `registerRoom` |
| `/rooms/{roomId}` | The name, the capacity, the opening hours, and the time zone of the room, above the reservations of Reservation | `getRoom` |

- The form shows room-name-taken as a notice about the name, invalid-opening-hours at the closing time, and unknown-time-zone at the time zone (`registration.ts`). Any other problem shows its title and detail as they are.
- A room that does not exist shows the page of `frontend/src/app/rooms/[roomId]/not-found.tsx`.
