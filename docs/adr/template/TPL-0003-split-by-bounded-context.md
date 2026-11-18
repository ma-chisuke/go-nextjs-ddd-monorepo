---
status: accepted
date: 2026-11-17
---

# TPL-0003: Split the code by bounded context, and let the kind of subdomain set the weight of the implementation

## Context and Problem Statement

The template must show how to organize the code of a business, and that organization must still hold after a team has replaced the sample with its own business.
Two questions decide most of the structure.

The first question is the main axis of the split.
A product grows by business capability: a new rule for bookings, a new kind of notice.
If the code is split by technical layer across the whole product, with all handlers in one place and all repositories in another, one business change reaches into every layer.
Nothing in such a layout shows which code belongs to which part of the business, and nothing stops one part from using the tables and types of another.

The second question is how much design each part gets.
The parts of a business differ in value and in complexity.
The rules for booking a meeting room are where the product differs from others; the list of rooms is plain data entry; sending an email is a problem that others have already solved.
If every part gets the full set of tactical patterns (aggregates, value objects, repositories, domain events), effort goes where it returns nothing.
If every part gets a thin script, the complex rules have no model, and they spread across handlers and queries.

How should the template split its code, and how much structure should each part get?

## Decision Drivers

- A business change stays inside one part of the code, and the name of that part is a word of the business.
- The amount of design follows the complexity of the business logic, not one rule for the whole product.
- The sample shows each style next to the other, so a team can copy the style that fits its own context.
- The code, the contracts, the schema, the documents, and the infrastructure are split the same way, so one name leads to all of them.

## Considered Options

- Split by bounded context, with the weight set by the kind of subdomain
- Split by technical layer across the whole product
- Split by bounded context, with the full set of tactical patterns in every context
- No split

## Decision Outcome

Chosen option: "Split by bounded context, with the weight set by the kind of subdomain", because it keeps a business change in one place, and it spends design effort only where the business logic is complex.

Every bounded context has the same name in each of these places.

| Part | Place |
|---|---|
| Go code | `backend/internal/<context>/`, with its entry points in `backend/cmd/<context>/` |
| Screens | `frontend/src/features/<context>/` |
| HTTP contract | `contracts/http/<context>.openapi.yaml` (TPL-0006) |
| Database | One schema for each context (TPL-0009) |
| Documents | `docs/contexts/<context>/`: a README that states the responsibilities, the public API, and the events, and a glossary of the business terms |
| Infrastructure | One Terragrunt unit for each context (TPL-0015) |

The kind of subdomain decides how a context is built, following the heuristics in "Design Heuristics", in Vlad Khononov, "Learning Domain-Driven Design".

| Kind | Implementation |
|---|---|
| Core subdomain | A domain model, with ports and adapters (TPL-0004) |
| Supporting subdomain | A transaction script: each use case calls the generated queries (TPL-0011), and there is no domain layer |
| Generic subdomain | A product or a library that is bought or taken off the shelf, used through a port |

- The README of each context states its kind and the reason for it.
- The heuristics also offer the active record pattern and the event-sourced domain model. The template uses one pattern for each kind, so a reader learns two styles, not four. A team that needs another pattern records it in a project record.

The sample business is the booking of meeting rooms.

| Context | Kind | Implementation | Responsibility |
|---|---|---|---|
| Reservation | Core | Domain model | Book, change, and cancel a meeting room |
| Facility | Supporting | Transaction script | Register meeting rooms, with their capacity and their opening hours |
| Notification | Supporting | Transaction script | Send an email when a booking is confirmed or canceled |

### Consequences

- Good, because a business change usually stays inside one directory of each part, and the name of that directory is the name of the context.
- Good, because the complex rules of the core subdomain get a model that tests can run without a database, while the simple contexts stay short.
- Good, because the sample shows both styles side by side, and the README of each context says why it got its style.
- Bad, because people and agents learn two styles, and must read the README of a context before they add code to it.
- Bad, because the kind of a subdomain is a judgment. The judgment can be wrong, or it can go out of date as the business changes.
- Bad, because a supporting context whose rules grow must be moved to a domain model, which means rewriting its business logic around aggregates.
- Bad, because each context has its own directories, contract, schema, and documents, which costs more files and more wiring than a single package, most of all in a context with little logic.

### Confirmation

Review enforces the classification, because no program can tell a core subdomain from a supporting one.
A reviewer compares a change with the README of each context that the change modifies.

These checks keep the split in the Go code:

- The Go `internal` directories and the import rules of `lint-arch` (TPL-0004, TPL-0005) keep the code of each context apart.
- sqlc, configured for each context, and the runtime role of each context, which has privileges on its own schema only (TPL-0009), keep each context inside its own schema (TPL-0005).
- `lint-arch`, with the rule `arch/glossary`, fails when a context has no glossary in English and in Japanese under `docs/contexts/<context>/`, when the glossary lacks an exported type of the `domain` package of the context, or of `app` in a context without one, and when it names an identifier that the package does not export.

The check that fails when a directory under `backend/internal/`, other than `platform/`, has no README under `docs/contexts/` is not built yet. Until it is, review enforces it.

The screens follow the same split. Each context with screens has a feature, `frontend/src/features/<context>/`, that holds the calls to its API, its Server Functions, its pure logic, and its components. A page in `frontend/src/app/` composes the features, and `frontend/src/app/_compose/` turns what one context gives into what another needs, as the adapter `adapter/facility` does in the backend: the page of a room hands Reservation a room of Facility in the words of Reservation.

- eslint-plugin-boundaries, which `lint-frontend` runs, keeps the features apart: a feature never imports another feature, and `frontend/src/lib/` imports no feature (rule `boundaries/dependencies`, in `frontend/eslint.config.mjs`). The self-test proves the rule with a feature that imports the server module of Facility (`case_lint_frontend`).
- No check keeps a feature to the API of its own context: `frontend/src/lib/api/server/clients.ts` gives the client of every API to every feature. Review keeps it.

## Pros and Cons of the Options

### Split by bounded context, with the weight set by the kind of subdomain

- Good, because the design effort goes where the business logic is complex.
- Good, because the boundaries in the code follow the boundaries of the business language.
- Bad, because there are two styles of code, and the choice between them is a judgment.

### Split by technical layer across the whole product

- Good, because many frameworks start with this layout, and most readers already know it.
- Bad, because a business change reaches into every layer, and nothing in the layout shows which code belongs to which part of the business.
- Bad, because nothing stops one part of the business from using the tables or the types of another, so the parts grow into each other.

### Split by bounded context, with the full set of tactical patterns in every context

- Good, because there is one style to learn.
- Bad, because a context that only stores and lists data gets aggregates, repositories, and mapping code that cost more than the logic they hold.
- Bad, because the sample would teach that every context needs the full set, which is the mistake that the heuristics are meant to prevent.

### No split

- Good, because a small product starts fastest as one package.
- Bad, because boundaries added later must be cut out of code whose parts already depend on each other.
- Bad, because the template would not show the structure that it exists to show.

## More Information

- "Design Heuristics", in Vlad Khononov, "Learning Domain-Driven Design" (O'Reilly, 2021): the choice of implementation pattern for each kind of subdomain.
- "Analyzing Business Domains", in the same book: core, supporting, and generic subdomains.
