---
status: accepted
date: 2026-11-17
---

# TPL-0012: Use UUIDv7 for primary keys

## Context and Problem Statement

Every entity needs an identity that does not change while the entity exists.
The type of the primary key is hard to change later, because the id spreads: into indexes and foreign keys inside the context, into API paths, into events, and into other contexts that store it.

Three needs shape the choice.

- The id must be known before the row exists. A use case that creates a reservation returns its URL in a `Location` header, and the event that announces the reservation carries its id. If the database assigns ids, the code must insert first and read the id back.
- In a core context, the domain uses no randomness and no I/O (TPL-0004), so it cannot generate ids itself. Something outside the domain must provide them, and tests must be able to fix them.
- The index of the primary key must stay efficient as the table grows.

Which type do primary keys use, and where are new ids created?

## Decision Drivers

- An id can be created anywhere, in the service, in a test, or before an event is written, without asking the database.
- Inserts into the index of the primary key stay local as a table grows.
- The id is a standard type that PostgreSQL, Go, TypeScript, and JSON Schema all know.
- The id does not reveal how many rows a table holds or how fast it grows.

## Considered Options

- UUIDv7 in a `uuid` column
- `bigint` identity
- UUIDv4
- ULID

## Decision Outcome

Chosen option: "UUIDv7 in a `uuid` column", because it can be created anywhere without coordination, keeps inserts into the index local as a sequence does, and is a standard (RFC 9562).

The rules are as follows.

- Every primary key is a UUID of version 7 (RFC 9562), in a column of type `uuid`, never `text`.
- In every context, the use case creates the id in Go, before it writes anything. In a core context, the use case gets the id from a port and passes it to the domain; the domain does not generate ids (TPL-0004), and tests give fixed ids. In a context that uses transaction scripts, the use case calls the same generator.
- Each primary key column also declares `DEFAULT uuidv7()`, with the function built into PostgreSQL 18, for rows that are inserted outside the application, such as seed data or a manual repair.
- In APIs and events, an id is the standard text form of a UUID.

### Consequences

- Good, because the values grow with time, so new rows land at one end of the B-tree index, as with a sequence, and the index stays compact.
- Good, because the code can know the id before the row exists: the use case has the id for the `Location` header and for the event before it writes anything.
- Good, because ids from different contexts, or from databases split apart later, do not collide, without any coordination.
- Good, because the id reveals nothing about the number of rows, unlike a sequence.
- Bad, because a UUID takes 16 bytes, twice a `bigint`, in every key, in every index that holds it, and in every reference to it.
- Bad, because the first 48 bits are the creation time in milliseconds, so anyone who sees an id learns when the entity was created. For most entities the API returns the creation time anyway; see More Information for entities whose creation time must stay private.
- Bad, because a UUID is long and hard to read, compare, or say aloud when a person looks at logs and URLs.

### Confirmation

M0 has no schema and no domain code, so nothing enforces this decision yet.
With the first bounded context, a check of each `schema.sql` fails when a primary key is not of type `uuid`.
Review checks that every use case creates new ids in Go, that a core context takes them from a port, and that its domain does not generate them.

## Pros and Cons of the Options

### UUIDv7 in a `uuid` column

- Good, because it is ordered by time, can be created anywhere, is standard, and has a native type and, since PostgreSQL 18, a built-in function (`uuidv7()`).
- Bad, because it takes 16 bytes, reveals the creation time, and is hard to read.

### `bigint` identity

- Good, because it takes 8 bytes, gives the most compact index, and is easy to read.
- Bad, because only the database assigns it, so the code learns the id only after the insert.
- Bad, because the ids are easy to guess and reveal volume: two ids seen a day apart tell anyone how many rows were created in between.
- Bad, because ids from two databases collide, so merging data from two databases needs new ids.

### UUIDv4

- Good, because its 122 random bits reveal nothing, not even the creation time, and every tool supports it.
- Bad, because random values spread inserts across the whole index: each insert writes to a different page, pages split more often, and as the table grows, less of the index stays in memory.

### ULID

- Good, because it is ordered by time like UUIDv7, and its text form is short (26 characters) and sorts in the same order.
- Bad, because it is a community specification, not a standard like RFC 9562, and PostgreSQL has no type or function for it. Stored in a `uuid` column, it has one text form in the database and another in the code.
- Bad, because RFC 9562 now standardizes the same idea as UUIDv7: a timestamp in milliseconds followed by random bits.

## More Information

The following guidance comes from the same analysis and applies to every context.

- An id that is hard to guess is not access control. Every request still checks that the caller may use the resource with that id, and an id that the caller may not see gets the same 404 as an id that does not exist.
- When the creation time of an entity must stay private (an anonymous report, or the sign-up date of a user), expose a separate UUIDv4 in its own column with a unique index, and keep the UUIDv7 primary key internal.
- A link that grants access by itself, such as a share link, a password reset, or an invitation, uses a separate random token from a cryptographic source, not an id.

References:

- [RFC 9562](https://www.rfc-editor.org/rfc/rfc9562.html), which defines UUID version 7
- [UUID functions](https://www.postgresql.org/docs/18/functions-uuid.html) in the PostgreSQL 18 documentation
