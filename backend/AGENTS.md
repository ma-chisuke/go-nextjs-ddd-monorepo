# AGENTS.md (backend)

Instructions for coding agents in `backend/`, in addition to the root [AGENTS.md](../AGENTS.md). [ARCHITECTURE.md](../ARCHITECTURE.md) is the map.

## Commands

| Command | Use |
|---|---|
| `make gen` | Regenerate the servers and clients from `contracts/http/` (oapi-codegen), and the typed queries from `schema.sql` and `queries/` (sqlc). |
| `make db-up` | Start the local PostgreSQL in Docker, and apply every `schema.sql`. |
| `make db-server` | Start the local PostgreSQL only if it does not run, and change nothing. Use it in a Git worktree: `make db-up` recreates a container that another checkout started. |
| `make db-plan`, `make db-apply` | Print, then run, the DDL that brings the local database to each `schema.sql`. |
| `make test-integration` | Run the tests behind the build tag `integration` against the local PostgreSQL, on the database `app_test` with every schema applied, and against AWS Emulator. |
| `make aws-server` | Start AWS Emulator only if it does not run, and copy its CA to `.cache/aws-emulator/ca.pem`. |
| `make aws-apply` | Create the event bus and the queues in the running emulator from `infra/local/`. After the emulator restarts, run it, then restart the relay and the workers. |
| `make gen-db-docs` | Rewrite `docs/generated/db/<context>/` after a change to a `schema.sql`. |

- `make db-down` deletes the local data of every checkout, because they share one database.
- Procedures: [add an endpoint](../docs/guides/add-an-endpoint.md), [change a schema](../docs/guides/change-a-schema.md), [change an event](../docs/guides/event-changes.md), [add a context](../docs/guides/add-a-context.md), [run locally](../docs/guides/run-locally.md); skills `add-endpoint`, `change-schema`, `change-event`, and `add-context`.

## Where code goes in a context

| Code | Place, under `internal/<context>/` |
|---|---|
| The wiring of adapters, use cases, and handler | `module.go`; only `cmd/<context>` imports it |
| Rules, events, and business errors of a core context | `internal/domain/` |
| One use case per file, the ports, and the errors of the use cases | `internal/app/` |
| The operations of the strict server, and the table from error to problem | `internal/adapter/http/` |
| `schema.sql`, `queries/`, the generated queries, and the store | `internal/adapter/postgres/` |
| The mapping from domain events to integration events, and the outbox | `internal/adapter/postgres/`: `events.go`, `outbox.go` |
| The handler of the events that a worker consumes, and the reading of their data | `internal/app/`, with `events.go`; `NewHandler` returns it as an `event.Handler` |
| The generated client of another context, and the translation into this context's words | `internal/adapter/<other>/` |
| The adapter of a port to a service outside, such as a mail server | `internal/adapter/<name>/`, such as `adapter/smtp/` |

Code that every context uses and no context owns goes in `internal/platform/`, such as the envelope of events (`event`), the relay (`outbox`), and the adapters for AWS (`awsconfig`, `awsevents`).

## Rules that the checks enforce

- `arch/context-import`: no import of a package of another context; call its API through a generated client, or use events. (TPL-0005)
- `arch/module-import`: only `cmd/<context>` imports the package of `module.go`. (TPL-0005)
- `arch/platform-import`: `platform/` imports no context. (TPL-0004)
- `arch/layer`: `adapter` imports `app` and `domain`, `app` imports `domain`, and adapters do not import each other. Only in a context without `domain` may `app` import its `adapter/postgres`. (TPL-0003, TPL-0004)
- `arch/domain-purity`, `arch/domain-clock`: the domain imports only an allow-list of the standard library, and reads no clock; it takes the time and new ids as arguments. (TPL-0004)
- `arch/route-registration`: no `Handle` or `HandleFunc` outside generated code and `platform/httpserver`; add the operation to the API document instead. (TPL-0007)
- `arch/schema-reference`: SQL names no schema, neither its own nor that of another context. (TPL-0005, TPL-0009)
- `arch/glossary`: each type that `domain` exports, or `app` in a context without `domain`, has a row in both glossaries of the context. (TPL-0003)
- `gen-check-api`, `gen-check-sqlc`: generated code is what `make gen` writes. (TPL-0007)
- exhaustive and `problems_test.go`: each kind of business error has a problem, and each problem a page in `docs/problems/`. (TPL-0008)
- `platform/apitest`: API tests go through its client, which validates every response against the API document. (TPL-0007)
- `lint-events`, `lint-events-breaking`: the JSON Schema of each event is well formed, has its message in the AsyncAPI document of its context, and changes only in ways that the consumers of its version can read; any other change is a new version. (TPL-0013)
- `eventtest.AssertMatchesContract`: the tests of a publisher pass each integration event through it. A domain event without an integration event fails the change, and the test of the mapping. (TPL-0013)
- `lint-sql-plan`, `lint-sqlc`, `lint-db-docs`, in `make check-all`: safe DDL for each schema change, queries that prepare, and comments on every table and column. (TPL-0010, TPL-0011, TPL-0023)
