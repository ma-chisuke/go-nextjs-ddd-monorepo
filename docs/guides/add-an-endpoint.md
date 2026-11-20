# Add an endpoint

An endpoint starts as an operation in the API document of its context, and it is finished when `make check` passes.
The steps name the files of a context `<context>`; [ARCHITECTURE.md](../../ARCHITECTURE.md) shows what each layer holds.
In a supporting context such as Facility, which has no domain layer, skip step 3.

## Steps

1. Describe the operation in `contracts/http/<context>.openapi.yaml`: its path, `operationId`, `summary`, parameters, body, and every response that it can give. Each 4xx and 5xx response refers to `#/components/responses/Problem`. The descriptions are in English.

   Check: `make check` runs `lint-contracts`, which fails on an operation without `operationId` and on an error response that is not `application/problem+json`, and `lint-contracts-breaking`. A new operation breaks no client; to change an existing one, read [Change an API](api-changes.md).

2. Generate the code:

   ```sh
   make gen
   ```

   oapi-codegen rewrites the server of the context, `internal/adapter/http/api.gen.go`, and every client of this API in other contexts, such as `internal/adapter/facility/client.gen.go` in Reservation. openapi-typescript rewrites the TypeScript types of the API for the frontend, `frontend/src/lib/api/<context>.gen.ts`.

   Check: the build fails with `missing method <Operation>`, because `handler.go` asserts that its operations implement the generated `StrictServerInterface`. `make check` reports it in `lint-go` and `test-go`. A change to an operation that the frontend calls also fails `lint-frontend-types` at each call that no longer fits the document; fix those calls in the same pull request.

3. In a core context, write the rules in `internal/domain/`. A new rule is a function or a method that takes values, the current time included, and returns values, events, and a `*domain.Error` of a new `Kind`, which you add to `Kinds()`. Test it with plain function calls.

   Check: `make check` fails when the domain imports a package outside its allow-list (`arch/domain-purity`) or reads the clock (`arch/domain-clock`).

4. Write the use case in `internal/app/`.
   In a core context, it is a method of `Reservations` in a file of its own: it loads what the domain needs through the ports of `ports.go`, calls the domain, and stores the result with the domain events that the domain returned, which the store writes to the outbox in the same transaction. A new type of domain event needs an integration event and its contract first: follow [Change an event](event-changes.md#add-an-event). A new need from outside becomes a method of a port, which an adapter implements.
   In a supporting context, it is a method that checks its input and calls the generated queries.
   An error of the use case that is not a rule of the domain is an `*app.Error` of a new `app.Kind`, which you add to `app.Kinds()`.

5. If no query reads or writes what the use case needs, add one to `internal/adapter/postgres/queries/<table>.sql` under a line `-- name: <Name> :one` (or `:many`, or `:exec`), and run `make gen` again.
   In a core context, implement the new method of the port in `adapter/postgres/store.go`, which maps rows to domain values.
   Add the method to the in-memory fakes that implement the same interface: `internal/app/apptest/` in Reservation, `internal/adapter/postgres/postgrestest/` in Facility.
   If a table must change, follow [Change a schema](change-a-schema.md) first.

   Check: `make check` runs `gen-check-sqlc`, and the compiler names each fake that lacks the method. `make check-all` also prepares the query against the schema (`lint-sqlc`) and runs the integration tests.

6. Implement the operation in `internal/adapter/http/handler.go`. The method that the compiler asks for maps the generated request to the input of the use case, calls it, and maps the result to the generated response. It returns a business error the way the other operations of the file do, and `problems.go` turns that error into its problem.

7. Give each new kind of business error a problem. In `internal/adapter/http/problems.go`, add a `problem.Type` with a new slug, a title, and a status that the operation declares in the API document, and a case for the kind to the switch. Write its page `docs/problems/<slug>.md` in the form of the other pages, and the translation `docs/problems/<slug>.ja.md`, then run `make i18n-stamp`.

   A rule that a JSON Schema cannot express, such as one across two properties, refuses some requests that the document allows. Add the status of its problem to `positive_data_acceptance` of the operation in `tools/schemathesis/schemathesis.toml`, with the rule as the reason; otherwise `test-api-fuzz` reports each such refusal (step 10).

   Check: `make check` fails while the switch lacks the kind (the linter exhaustive, in `lint-go`), and while the page is missing (the test in `problems_test.go`).

8. Test the operation in `internal/adapter/http/handler_test.go`, through the client of `platform/apitest`: the request that succeeds, and a request for each problem that the operation can answer.

   Check: `make check` runs the tests. The client validates every response against the API document, so an undocumented status, a missing header, or a body of the wrong shape fails the test.

9. Update the documents of the context: the table of operations in `docs/contexts/<context>/README.md`, and a row in `glossary.md` for each type that the change exports from `domain`, or from `app` in a supporting context. Change the translations as well, then run `make i18n-stamp`.

   Check: `make check` fails while an exported type has no row in both glossaries (`arch/glossary`), and while a translation is out of date (`lint-i18n`).

   To show the operation on a screen, follow [Add a screen](add-a-screen.md).

10. Finish:

    ```sh
    make fmt
    make check
    CHECK_SCOPE=changed make check-all   # adds the checks that need PostgreSQL (Docker)
    ```

    Check: `make check-all` runs `test-api-fuzz`, which sends requests that Schemathesis generates from the document to the API, built from the checkout: the examples of the document, requests that test each constraint from both sides, generated requests, and sequences of them. A finding is an answer that the document does not allow, or a 5xx. [Change an API](api-changes.md#check-the-api-against-requests-generated-from-its-document) tells how to read a finding, send its request again, and decide which side to fix.
