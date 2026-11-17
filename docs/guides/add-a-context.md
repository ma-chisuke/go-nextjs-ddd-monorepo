# Add a context

A bounded context has the same name in the code, the API document, the database, and the documents (TPL-0003).
Choose that name first: one lowercase word, such as `facility`, because it becomes a Go package, a schema, and the prefix of two roles.
Then decide the kind of subdomain, because it decides what to copy: Facility for a supporting context, a transaction script; Reservation for a core context, a domain model with ports and adapters ([ARCHITECTURE.md](../../ARCHITECTURE.md)).

## Steps

1. Write the API document `contracts/http/<context>.openapi.yaml`. Copy the parts of `facility.openapi.yaml` that every API shares: `info`, `servers`, `security: []` with its comment (only the private network reaches the API, TPL-0014), the response `Problem`, and the schemas `Problem` and `InvalidParam`. Then describe the operations.

   Check: `make check` runs `lint-contracts`.

2. Copy the code of the model context to `backend/internal/<context>/`, and rename what carries the old name:

   - `module.go`, with `Deps` and `NewHandler`;
   - `internal/app/`: the use cases and the business errors, and in a core context the ports and the in-memory fakes in `app/apptest/`;
   - `internal/domain/`, in a core context only;
   - `internal/adapter/http/`: `oapi-codegen.yaml` unchanged, `handler.go`, `problems.go`, and `problems_test.go`;
   - `internal/adapter/postgres/`: `schema.sql`, `queries/`, and `sqlc.yaml`, whose settings `database`, `analyzer`, and `rules`, and the rule `no-unfiltered-write` at its end, stay as they are, because `lint-sqlc` relies on them.

   Leave out the generated `*.gen.go` files, and run `make gen`. `gen-backend.sh` finds the API document of `adapter/http/` by the name of the context.
   To call the API of another context, add `internal/adapter/<other>/oapi-codegen.yaml` with `client: true` and `models: true`, as in Reservation's `adapter/facility/`. `make gen` reads `contracts/http/<other>.openapi.yaml` for it.

   Check: `make check`. `lint-arch` derives every rule from the directories, so the new context needs no configuration.

3. Write the command `backend/cmd/<context>/main.go` from `cmd/facility/main.go`. It reads `DATABASE_URL`, `HTTP_ADDR`, and `LOG_LEVEL`, imports `time/tzdata`, and calls `<context>.NewHandler`. Only this command may import the package of `module.go`.

   Check: `make check` fails when another package imports it (`arch/module-import`).

4. Give the context its schema and roles. Copy the block of Facility in `db/bootstrap.sql`, and replace `facility` with the name of the context: the roles `<context>_owner` and `<context>_runtime`, the schema that the owner role owns, the privileges of the runtime role on it, and the search path `<context>, extensions` of both roles. Add the passwords of the two roles to `db/local.sql`. Then write `schema.sql` as [Change a schema](change-a-schema.md) describes, with the `GRANT` to `<context>_runtime` on each table.

   ```sh
   make db-up        # runs both files again, and applies every schema
   make gen-db-docs  # writes docs/generated/db/<context>/
   ```

   PostgreSQL runs the two files by itself only when it creates its data directory, so on a local database that exists already, `make db-up` runs them.

5. Write the glossary, `docs/contexts/<context>/glossary.md` and `glossary.ja.md`: one table with the header `| Identifier | Term | Meaning |` (`| 識別子 | 用語 | 意味 |` in Japanese), and a row for each type that `domain` exports, or `app` in a supporting context.

   Check: `make check` fails on each exported type without a row (`arch/glossary`).

6. Write the README of the context, `docs/contexts/<context>/README.md` and its translation, in the form of those of Facility and Reservation: what it is responsible for, its kind of subdomain and why, its API and problems, its events, its rules, and what it needs from other contexts. Add the context to the tables of `ARCHITECTURE.md`.

7. Write a page in `docs/problems/` for each new slug of `problems.go`, and run `make i18n-stamp`.

8. Finish:

   ```sh
   make fmt
   make check
   CHECK_SCOPE=changed make check-all
   ```

## What the checks tell you is missing

| Missing | The check that tells |
|---|---|
| The API document | `make gen` and `gen-check-api` (`gen/no-document`) |
| An operation in `handler.go` | The compiler, in `lint-go` and `test-go` |
| Generated code that is current | `gen-check-api`, `gen-check-sqlc` |
| A row of the glossary, or a glossary | `lint-arch` (`arch/glossary`) |
| A translation | `lint-i18n` |
| A problem for a kind of business error, or its page | `lint-go` (exhaustive), and the test in `problems_test.go` |
| The block of `sqlc.yaml` that prepares the queries | `lint-sqlc` (`sqlc/no-db-prepare`) |
| The roles or the schema in `db/bootstrap.sql`, or the passwords in `db/local.sql` | `make db-up`, and each check of `make check-all` that applies the schema (`pgschema/apply`) |
| A `GRANT` to the runtime role | The integration tests of the context, which connect as that role (`test-integration`) |
| A comment on a table or a column, or the schema documents | `lint-db-docs` |
| The README of the context | No check yet; review (TPL-0003) |
