---
status: accepted
date: 2026-11-20
---

# TPL-0007: Keep the OpenAPI documents and the implementation in agreement with layered checks

## Context and Problem Statement

The OpenAPI document of each bounded context is the source of its API (TPL-0006).
The template requires that the inputs and outputs of the implementation cannot drift from the document without a machine noticing.

The generated types catch much of the drift, but not all of it.

- Generated code that nobody regenerated after a change of the document still compiles.
- A route registered by hand does not appear in the document.
- Types cannot express every constraint. A Go type says that a value is a string; only the document says which strings are valid, through a pattern, a maximum length, or a format such as `date-time`.
- The body of an error response can have any shape, and nothing in the types forbids that.
- A request that no test sends can still reach code that breaks the document.
- A change to the document can break the clients that use it, while the document itself stays valid.

No single tool catches all of these.
Which checks, together, make each kind of drift visible, and where does each check run?

## Decision Drivers

- A machine catches each kind of drift before the change merges.
- Each check catches what the earlier ones cannot, so a mistake usually gives one finding (TPL-0023).
- The fast checks run in `make check`; the slow ones run in `make check-all` and in CI.
- Production pays only for the checks that protect it.

## Considered Options

- Layered checks, each catching what the one before cannot
- Generated types only
- Consumer-driven contract tests (Pact)
- Layered checks, with responses validated in production as well

## Decision Outcome

Chosen option: "Layered checks, each catching what the one before cannot", because each kind of drift in the list above has a check that fails on it, and production runs only the validation of requests.

| Layer | What it catches | How |
|---|---|---|
| 1. Compile time | A missing operation, a wrong parameter, a wrong response type | The strict server interface of oapi-codegen in Go; `tsc` on the types that openapi-typescript generates |
| 2. Freshness | Generated code that is older than the document | Regenerate, and require an empty diff |
| 3. No entry point outside the document | A route registered by hand | Only generated code registers routes; a lint rule forbids registering a route directly |
| 4. Validation in tests | Constraints that types cannot express, and the shape of errors | Tests validate every request and every response against the document with kin-openapi |
| 5. Generated inputs | Inputs that nobody wrote a test for | Schemathesis generates requests from the document, the same requests in every run |
| 6. Compatibility | A breaking change for the existing clients | oasdiff compares the document with the version on the main branch |
| 7. The document itself | A document that breaks the rules of the template | Redocly CLI lints the document, including a rule that 4xx and 5xx responses use only `application/problem+json` (TPL-0008) |

- In production, a middleware validates each request against the document with kin-openapi, and answers an invalid request with a problem (TPL-0008).
- Production does not validate responses. Layer 4 checks them in tests before release, and validating each response in production would add work to every request for drift that the tests already look for.
- Layer 5 needs running APIs and a database, and it is slow, so it runs in `make check-all`. Each run of the same commit sends the same inputs, so a failure can be repeated.

Layer 5 works as follows.

- `test-api-fuzz` builds the API of each context from the checkout, and starts each API on a free port against a database of its own on the local PostgreSQL, `api_fuzz_<checksum of the checkout path>`, which it creates before the run and drops after it. It then runs Schemathesis 4.33.4 on each document of `contracts/http/`. With `max-examples = 100`, Schemathesis sends about 1,900 requests, 479 to Facility and 1,398 to Reservation, in about half a minute on a quiet machine.
- The first plan said "with a fixed seed". What makes two runs the same is more than a seed: the deterministic mode of Schemathesis, which takes a fixed seed from each test and ignores `--seed`, no database of examples, the cache off, a fresh database, an empty working directory, and one worker. Two runs send the same requests; only the ids that the APIs create differ.
- No check of Schemathesis is off. One of them, that a request which the document allows is not refused (`positive_data_acceptance`), also accepts 422 on the operations whose rules a JSON Schema cannot state (registerRoom, listRooms, bookRoom, listReservations), and 412, 422, and 428 on rescheduleReservation and cancelReservation, where `If-Match` decides. `tools/schemathesis/schemathesis.toml` gives the reason for each setting.

### Consequences

- Good, because each kind of drift in the problem statement has a check that fails on it.
- Good, because the layers catch different things, so a mistake usually gives one finding, not seven.
- Good, because the tests check the shape of every error, along with every other response, without a separate review.
- Good, because layer 5 found two faults on its first run that the tests had missed. A 405 answer had no `Allow` header, which RFC 9110 requires, and a whole number written with a fraction, such as `4.0`, was refused with 400, although JSON Schema 2020-12, which OpenAPI 3.1 uses, counts it as an integer. Both were in the shared request validation, `platform/httpserver/validate.go`, and both are fixed. Layer 4 now also fails a 405 without `Allow`.
- Bad, because the template pins and updates several tools for API conformance, one for each kind of drift: kin-openapi, Schemathesis, oasdiff, and Redocly CLI, next to the generators.
- Bad, because layer 4 covers only the requests that tests send. Layer 5 narrows the gap, but no layer proves that every response of every operation matches the document.
- Bad, because layer 5 covers the surface of the Reservation API, not its rules. A generated booking names a room that does not exist, so the API refuses it before the rules of the domain see it, and the stateful phase follows none of the links that Schemathesis infers. Layer 4 covers the rules.
- Bad, because the request validation answers with 422 too, so accepting 422 on the operations above could hide a disagreement about a format between the validator of the API and Schemathesis.
- Bad, because a deliberate breaking change fails layer 6, unless the same change raises the version of the document.
- Bad, because layer 5 builds and starts the APIs and needs the local PostgreSQL, which makes `make check-all` slower. On a very busy machine, Reservation can answer 503 when Facility is slow, and layer 5 reports that as a server error.

### Confirmation

All seven layers check the Go side now.
Layer 1 compiles each operation against the strict server interface (`lint-go`, `test-go`), and layer 2 is `gen-check-api`, with `gen-check-sqlc` next to it for the code that sqlc generates.
Layer 3 is the rule `arch/route-registration` of `lint-arch`.
Layer 4 is the client of the API tests, `backend/internal/platform/apitest`, which validates every response with kin-openapi, together with the request validation of `platform/httpserver`, which every request of a test passes through.
Layer 5 is `test-api-fuzz` (`tools/scripts/api-fuzz.sh`), which prints each finding as one line at the line of the operation in the document.
Layer 6 is `lint-contracts-breaking`, and layer 7 is `lint-contracts`.
Since the frontend milestone, layers 1 and 2 check the TypeScript side too. The Next.js app calls the APIs through openapi-fetch, which takes the types that openapi-typescript generates from each document (TPL-0006), so `lint-frontend-types` fails on a call that the document does not allow. The self-test proves it with a query parameter that the document of Facility does not declare (`case_lint_frontend_types__api`). `gen-check-api-ts` is layer 2 for those types.
The frontend reads the answers of the APIs as their types say, and does not validate them at run time, as production validates no response; layer 4 checks the responses in the tests of each API. The end-to-end test (`make e2e`, in `make check-all`) then runs the screens against the real APIs.

The table names the target that runs each layer.

| Layer | Make target | Run by |
|---|---|---|
| 1. Compile time | `lint-go` and `test-go` for Go; `lint-frontend-types` for `tsc --noEmit` | `make check` |
| 2. Freshness | `gen-check-api`: runs oapi-codegen again, and fails when the result differs from the committed code; `gen-check-api-ts`: the same for openapi-typescript | `make check` |
| 3. No entry point outside the document | `lint-arch`, with the rule `arch/route-registration` | `make check` |
| 4. Validation in tests | `test-go`, through the client of `platform/apitest` | `make check` |
| 5. Generated inputs | `test-api-fuzz`, planned as `test-api`: Schemathesis on each document of `contracts/http/`, against the APIs built from the checkout | `make check-all` |
| 6. Compatibility | `lint-contracts-breaking`: oasdiff, against the same document at the merge base with `origin/main`, or with `CHECK_BASE`, which CI sets to the base branch | `make check` |
| 7. The document itself | `lint-contracts` | `make check` |

Layer 6 accepts a breaking change when the same change raises `info.version` of the document: below 1.0 the minor version, from 1.0 on the major version. [Change an API](../../guides/api-changes.md) describes this for the people who meet the finding, and tells how to read and fix a finding of layer 5.

The layers apply to the OpenAPI documents. The contracts of events, their JSON Schemas and AsyncAPI documents, have checks of the same kinds, which TPL-0013 lists: `lint-contracts` on the documents, `lint-events` on the schemas, `lint-events-breaking` for compatibility, and tests that validate every event against its schema.

## Pros and Cons of the Options

### Layered checks, each catching what the one before cannot

- Good, because every kind of drift has a check, and each check is aimed at a kind of drift that the others miss.
- Bad, because there are seven layers to understand, and a contributor must learn which layer failed and why.

### Generated types only

- Good, because it costs nothing beyond the generators of TPL-0006.
- Bad, because it misses stale generated code, routes added by hand, constraints that types cannot express, the shape of errors, and breaking changes.

### Consumer-driven contract tests (Pact)

- Good, because each consumer states what it relies on, and the provider is tested against exactly that.
- Bad, because the expectations of the consumers become a second contract next to the OpenAPI document, and the two can disagree.
- Bad, because it needs a place to share the contracts, such as a Pact Broker, and a test suite in each consumer, while the consumers here are already generated from the same document in the same repository.

### Layered checks, with responses validated in production as well

- Good, because drift in an operation that no test covers shows up in production.
- Bad, because every response costs extra work, and a response that fails validation must either be replaced by an error that the client did not need, or only be logged.
- Bad, because it finds drift after release, while layers 4 and 5 find most of it before.

## More Information

- [kin-openapi](https://github.com/getkin/kin-openapi)
- [Schemathesis](https://github.com/schemathesis/schemathesis)
- [oasdiff](https://github.com/oasdiff/oasdiff)
- [Redocly CLI](https://redocly.com/docs/cli)
