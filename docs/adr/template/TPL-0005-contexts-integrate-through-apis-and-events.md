---
status: accepted
date: 2026-11-17
---

# TPL-0005: Bounded contexts do not depend on each other, and integrate through HTTP APIs and events

## Context and Problem Statement

Bounded contexts (TPL-0003) are useful only while their boundaries hold.
A boundary breaks in small steps.
A context imports another context's type to save a mapping, reads another context's table to save an API call, or adds a foreign key to keep two tables consistent.
Each step looks cheap.
Together, the steps tie the models to each other: a change in one context breaks another, and the contexts can no longer be deployed apart.

In this repository the shortcuts are close at hand.
All contexts share one Go module (TPL-0002) and one database (TPL-0009), so the code and the tables of every other context are one import or one query away.

The contexts of the sample also need each other.
Reservation must know the capacity and the opening hours of a room, which Facility owns.
Notification must learn when a booking is confirmed or canceled, which Reservation decides.

How do the contexts exchange data without depending on each other's code and storage?

## Decision Drivers

- A context can change its model and its tables without breaking another context.
- Each context runs as its own service (TPL-0014), so whatever it calls must be reachable over the network.
- A program checks the rule, because each shortcut is one line of code away.
- The integration uses published standards and generated code, not a protocol written by hand.

## Considered Options

- HTTP APIs and integration events, with no direct dependency
- In-process calls between the modules of one program (a modular monolith)
- Shared tables, or joins across schemas
- A shared kernel library

## Decision Outcome

Chosen option: "HTTP APIs and integration events, with no direct dependency", because the compiler, the code generator, and the privileges of the database enforce its boundary, not convention, and each context can be changed, tested, and deployed alone. The separate deployments of TPL-0014 rely on exactly that.

The rules are as follows.

- A context never imports the Go packages of another context.
- A context never reads or writes the tables of another context, and no foreign key crosses from one context to another.
- For a synchronous need, a context calls the HTTP API of the other context, through a client generated from that context's OpenAPI document (TPL-0006). It translates the answer into its own model in an anticorruption layer, which lives in its adapters behind a port (TPL-0004).
- A context that provides an API offers it as an open-host service. The API and its OpenAPI document are a published language, kept apart from the internal model, so the model can change without a change to the API.
- For asynchronous integration, a context publishes integration events, and other contexts subscribe to them (TPL-0013).

In the sample, Reservation asks Facility for the capacity and the opening hours of a room over HTTP, and translates them into its own terms.
Reservation tells Notification about confirmed and canceled bookings through events.

The contexts use HTTP even though they share one repository, because each context runs as its own service on ECS (TPL-0014).
A function call from one context into another is not possible at run time, and an in-process call during development would hide the latency and the failures that production has.

### Consequences

- Good, because a context can rename, split, or reshape its tables and its domain without a change in another context, as long as its API and its events stay compatible.
- Good, because every dependency between contexts is visible in two places: the generated clients and the event subscriptions.
- Good, because a call between contexts uses the same kind of contract and the same checks as the API that the screens use (TPL-0007).
- Bad, because a call between contexts can fail or time out. The caller needs timeouts, retries, and idempotent operations, and it must decide what to do while the other context is down.
- Bad, because data that arrives through events is eventually consistent: for a short time, a context acts on data that is already out of date.
- Bad, because a context keeps copies of the data that it needs from others, and it must keep those copies up to date.
- Bad, because a screen or a report that combines data from two contexts must call both APIs or keep a copy. A join across schemas is not allowed.
- Bad, because a small team writes contracts, clients, and translations even for small exchanges, which is ceremony that a single program would not need.

### Confirmation

Each check catches a different shortcut, at a different moment.

| Shortcut | Check | Fails at |
|---|---|---|
| Import of another context's layers | Go `internal` directories: only code inside `backend/internal/<context>/` can import `backend/internal/<context>/internal/...` | Compile time |
| Import of any package of another context, in code or in tests | `lint-arch`, rule `arch/context-import` | `make check` |
| Import of the root package of another context, the one that holds `module.go` | `lint-arch`, rule `arch/module-import`: only `cmd/<context>` imports it | `make check` |
| A query on another context's table | sqlc, configured for each context, reads only that context's `schema.sql` (TPL-0011) | Code generation |
| Access to another context's schema while running | The runtime role of each context, with privileges on its own schema only (TPL-0009) | Run time |
| A foreign key across contexts | `lint-arch`, rule `arch/schema-reference`: no `schema.sql` and no file under `queries/` of a context names the schema of another context | `make check` |

The `internal` directories alone are not enough.
The root package of a context sits directly in `backend/internal/`, so any package under `backend/` may import it.
`lint-arch` closes that gap.

## Pros and Cons of the Options

### HTTP APIs and integration events, with no direct dependency

- Good, because the contexts can be deployed, scaled, and changed apart.
- Good, because the contracts between contexts are explicit, and the same checks guard them as the API of the screens.
- Bad, because every call between contexts crosses the network, with its latency and its failures.

### In-process calls between the modules of one program (a modular monolith)

- Good, because a call is a function call: no network, no serialization, and one transaction can span modules.
- Good, because it is the simplest start for a small team, and the modules can still keep their code apart.
- Bad, because it does not fit separate deployments (TPL-0014). Moving to them later turns every call into a network call, with failures that the callers never had to handle.
- Bad, because one transaction across modules ties their data together in a way that is hard to undo.

### Shared tables, or joins across schemas

- Good, because one query gets the data of two contexts at once, and the result is always consistent.
- Bad, because the table becomes a contract that nobody declared. A change of a column in one context breaks the queries of another, and no check sees it before run time.
- Bad, because it rules out giving a context its own database later.

### A shared kernel library

- Good, because types that two contexts agree on are written once.
- Bad, because a change to the library needs the agreement of every context that uses it, and every one of them is built and deployed again.
- Bad, because such a library tends to grow into a common model, which is the coupling that bounded contexts exist to avoid.

## More Information

- "Integrating Bounded Contexts", in Vlad Khononov, "Learning Domain-Driven Design" (O'Reilly, 2021): shared kernel, anticorruption layer, open-host service, and published language.
