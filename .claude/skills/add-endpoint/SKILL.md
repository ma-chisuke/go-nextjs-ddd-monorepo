---
name: add-endpoint
description: Add an HTTP endpoint to a bounded context, from the operation in contracts/http/<context>.openapi.yaml to the handler, the use case, the problems, and the API tests. Use when a change adds an operation to an API, or changes what an operation accepts or answers.
---

# Add an endpoint

Follow `docs/guides/add-an-endpoint.md`. It has the steps, the files, and the check that confirms each step; this skill adds only what to verify.

## Steps

1. Read the README of the context in `docs/contexts/<context>/`: whether it is core (domain model) or supporting (transaction script) decides which layers you touch.
2. Change the API document first, and run `make gen` before you write any Go. It regenerates the Go server, the clients of other contexts, and the TypeScript types of the frontend (`frontend/src/lib/api/<context>.gen.ts`). Never edit `*.gen.go` or `*.gen.ts`.
3. If the change breaks an existing client, read `docs/guides/api-changes.md` before you go on. `make check` then fails in `lint-frontend-types` at each call of the frontend that the change breaks: fix those screens in the same change.
4. Follow the guide step by step.
5. Verify, before you finish:
   - every status that the operation can answer is declared in the API document, and the API tests send a request for each problem;
   - each new kind of business error has a case in `problems.go` and a page `docs/problems/<slug>.md` with its translation;
   - each newly exported type of `domain` (or of `app` in a supporting context) has a row in both glossaries;
   - the table of operations in the README of the context, and its translation, name the operation.
6. Run `make fmt`, then `make check`. Run `make lint-sqlc test-integration test-api-fuzz` too: they need Docker, take about a minute, and try the queries, the integration tests, and requests generated from the API document. Do not wait for `make check-all`: it takes ten minutes or more, and CI runs it.
7. If a screen is to show the operation, continue with the skill `add-screen`.
