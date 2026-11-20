# Add a context

A bounded context has the same name in the code, its contracts, the database, and the documents (TPL-0003): the API document of a context with an API, the AsyncAPI document of a context that publishes or consumes events.
Choose that name first: one lowercase word, such as `facility`, because it becomes a Go package, a schema, and the prefix of two roles.
Then decide the kind of subdomain, and whether the context answers requests or only consumes events, because these decide what to copy ([ARCHITECTURE.md](../../ARCHITECTURE.md)):

- Facility, for a supporting context with an API: transaction scripts.
- Reservation, for a core context: a domain model with ports and adapters.
- Notification, for a context without an API: a worker that consumes the events of other contexts, with transaction scripts.

## Steps

1. Write the contracts of the context.

   For an API, write the API document `contracts/http/<context>.openapi.yaml`. Copy the parts of `facility.openapi.yaml` that every API shares: `info`, `servers`, `security: []` with its comment (only the private network reaches the API, TPL-0014), the response `Problem`, and the schemas `Problem` and `InvalidParam`. Then describe the operations.

   For a context that consumes events, write `contracts/events/<context>.asyncapi.yaml` in the form of `notification.asyncapi.yaml`: a `receive` operation for each event that it consumes, which refers to the channel and the message in the document of the publisher. A worker has no API document. [Change an event](event-changes.md#subscribe-to-events) describes the rest of a subscription.

   Check: `make check` runs `lint-contracts` on both kinds of document.

2. Copy the code of the model context to `backend/internal/<context>/`, and rename what carries the old name:

   - `module.go`, with `Deps` and `NewHandler`. The `NewHandler` of an API returns its `http.Handler`; that of a worker returns an `event.Handler` (`platform/event`), as in Notification;
   - `internal/app/`: the use cases and the business errors, and the ports and the in-memory fakes in `app/apptest/`, in a core context and in a supporting context that has a port, such as the `Mailer` of Notification;
   - `internal/domain/`, in a core context only;
   - `internal/adapter/http/`, in a context with an API: `oapi-codegen.yaml` unchanged, `handler.go`, `problems.go`, and `problems_test.go`;
   - `internal/adapter/postgres/`: `schema.sql`, `queries/`, and `sqlc.yaml`, whose settings `database`, `analyzer`, and `rules`, and the rule `no-unfiltered-write` at its end, stay as they are, because `lint-sqlc` relies on them.

   Leave out the generated `*.gen.go` files, and run `make gen`. For a context with an API, `gen-backend.sh` finds its API document from the directory `adapter/http/` and the name of the context, and `gen-frontend.sh` writes the TypeScript types of the API for the frontend, `frontend/src/lib/api/<context>.gen.ts`. To give the context screens, follow [Add a screen](add-a-screen.md).
   To call the API of another context, add `internal/adapter/<other>/oapi-codegen.yaml` with `client: true` and `models: true`, as in Reservation's `adapter/facility/`. `make gen` reads `contracts/http/<other>.openapi.yaml` for it.
   A worker reads the data of each event in an anticorruption layer of its own, as `internal/app/events.go` of Notification does, and records the id of each event that it handled under a unique constraint, in the transaction of its effect, because delivery is at least once (TPL-0013).
   For a core context, add its `internal/domain` and `internal/app` to `MUTATION_PACKAGES` in `make/go.mk`, so that `test-mutation` checks that their tests notice a change to the code ([Mutation tests](mutation-tests.md)).

   Check: `make check`. `lint-arch` derives every rule from the directories, so the new context needs no configuration.

3. Write the command `backend/cmd/<context>/main.go`. Only this command may import the package of `module.go`.

   - For an API, start from `cmd/facility/main.go`. It reads `DATABASE_URL`, `HTTP_ADDR`, and `LOG_LEVEL`, imports `time/tzdata`, and serves `<context>.NewHandler` with `httpserver.Run`.
   - For a worker, start from `cmd/notification/main.go`. It also reads the queue (`EVENTS_QUEUE_URL`) and the region (`AWS_REGION`), loads the configuration of the AWS SDK with `awsconfig.Load`, and runs the consumer of `platform/awsevents` with the handler of the context inside `httpserver.RunWorker`, which serves the health check next to it.
   - A worker next to an API is a mode of the same command, as `relay` is for Reservation: pass `httpserver.Mode{Name: "<mode>", Run: ...}` to `httpserver.Main`.

   Check: `make check` fails when another package imports the package of `module.go` (`arch/module-import`).

4. Give the context its schema and roles. Copy the block of Facility in `db/bootstrap.sql`, and replace `facility` with the name of the context: the roles `<context>_owner` and `<context>_runtime`, the schema that the owner role owns, the privileges of the runtime role on it, and the search path `<context>, extensions` of both roles. Add the passwords of the two roles to `db/local.sql`. Then write `schema.sql` as [Change a schema](change-a-schema.md) describes, with the `GRANT` to `<context>_runtime` on each table.

   ```sh
   make db-up        # runs both files again, and applies every schema
   make gen-db-docs  # writes docs/generated/db/<context>/
   ```

   PostgreSQL runs the two files by itself only when it creates its data directory, so on a local database that exists already, `make db-up` runs them.
   In a Git worktree, `make db-up` recreates the container that another checkout started. Run `make db-server` instead, and give the work a database of its own: `PGDATABASE=<name> tools/scripts/db.sh init` creates that database, runs the two files, and applies every schema to it.

5. Run a worker in the local system. In `compose.yaml`, add a service in the form of `notification`: the image built from `backend` with the build argument `CONTEXT`, the environment of `x-aws-emulator`, the volume `aws-emulator` for the CA, and the URL of its queue. In `infra/local/main.tf`, subscribe the context to the events that it consumes, as [Change an event](event-changes.md#subscribe-to-events) describes.

   Check: `make up` starts the worker and waits until it is healthy.

6. Give the context its units in AWS (TPL-0014, TPL-0015). Every environment has the same units, so each change below is made once, for stg and prd together.

   - Add the name of the context to `contexts` in `infra/live/system.hcl`. From that list, the unit `registry` creates the repository of its image, and the unit `database` the secrets of its two roles, whose passwords the task that prepares the database sets.
   - Write `infra/live/_units/contexts/<context>.hcl`: copy the unit of the model context, and change `name`. Keep what the context runs and calls: `api` for an API; `workers` for each worker, with its mode as `command`, such as `["relay"]`, or `[]` for a context without an API; `publishes_events = true` for a publisher; and `subscription`, with the sources and the types of the events that its AsyncAPI document receives, as in `infra/local/main.tf`. A permission beyond these goes into `iam_statements`, such as `ses:SendEmail` for Notification.
   - Copy a `terragrunt.hcl` of the tree to `infra/live/<environment>/contexts/<context>/terragrunt.hcl`, for every environment. Every `terragrunt.hcl` is the same file.
   - To call the API of another context, add a `dependency` on its unit and put its `api_url` in `environment_variables`, as the unit of Reservation does for Facility. If the frontend calls the new API, do the same in `infra/live/_units/web.hcl`.
   - The roles of step 4 are what the task that prepares the database creates in AWS, as the master user of Aurora, which is not a superuser. The block of Facility in `db/bootstrap.sql` makes that user a member of the owner role; keep that part when you copy it.

   ```sh
   make check                           # lint-infra-live
   CHECK_SCOPE=changed make check-all   # validate-live, rehearse-infra, test-db-image
   ```

   Check: `lint-infra-live` fails until the command in `backend/cmd/<context>/` is in `contexts`, has `_units/contexts/<context>.hcl`, and has a unit in every environment. [Deploy to AWS](deploy.md#change-an-environment) describes the rest of the tree.

7. Write the glossary, `docs/contexts/<context>/glossary.md` and `glossary.ja.md`: one table with the header `| Identifier | Term | Meaning |` (`| 識別子 | 用語 | 意味 |` in Japanese), and a row for each type that `domain` exports, or `app` in a supporting context.

   Check: `make check` fails on each exported type without a row (`arch/glossary`).

8. Write the README of the context, `docs/contexts/<context>/README.md` and its translation, in the form of those of Facility, Reservation, and Notification: what it is responsible for, its kind of subdomain and why, its API and problems or that it has none, its events, its rules, and what it needs from other contexts. Add the context to the tables of `ARCHITECTURE.md`, and its processes to the table of processes.

9. For a context with an API, write a page in `docs/problems/` for each new slug of `problems.go`. Then run `make i18n-stamp`.

10. Finish:

   ```sh
   make fmt
   make check
   CHECK_SCOPE=changed make check-all
   ```

## What the checks tell you is missing

| Missing | The check that tells |
|---|---|
| The API document of a context with an API | `make gen` and `gen-check-api` (`gen/no-document`) |
| An operation in `handler.go` | The compiler, in `lint-go` and `test-go` |
| Generated code that is current | `gen-check-api`, `gen-check-sqlc`, `gen-check-api-ts` (`gen/missing` for the TypeScript types of a new API document) |
| A row of the glossary, or a glossary | `lint-arch` (`arch/glossary`) |
| A translation | `lint-i18n` |
| A problem for a kind of business error, or its page | `lint-go` (exhaustive), and the test in `problems_test.go` |
| The block of `sqlc.yaml` that prepares the queries | `lint-sqlc` (`sqlc/no-db-prepare`) |
| The roles or the schema in `db/bootstrap.sql`, or the passwords in `db/local.sql` | `make db-up`, and each check of `make check-all` that applies the schema (`pgschema/apply`) |
| A `GRANT` to the runtime role | The integration tests of the context, which connect as that role (`test-integration`) |
| A comment on a table or a column, or the schema documents | `lint-db-docs` |
| The JSON Schema of an event that the context publishes, or its message in the AsyncAPI document | `lint-events` |
| The context in `contexts` of `infra/live/system.hcl`, the definition of its unit in `infra/live/_units/contexts/`, or its unit in an environment | `lint-infra-live` |
| An output that a unit reads and another unit does not have, or an input that a module rejects | `validate-live`, `rehearse-infra` |
| The AsyncAPI document of a consumer, or its subscription in `infra/local/main.tf` or in its unit | No check yet; review (TPL-0013). `make smoke` notices a missing subscription only if it waits for what the worker does |
| The README of the context | No check yet; review (TPL-0003) |
