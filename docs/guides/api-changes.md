# Change an API

The OpenAPI document of a context, in `contracts/http/`, is its contract with the clients that call it (TPL-0006).
The check `lint-contracts-breaking` compares each document with its version on the base branch, and fails on a change that breaks those clients (TPL-0007, layer 6).
The check `test-api-fuzz` sends requests generated from each document to the API, and fails on an answer that the document does not allow (layer 5).

## Make a breaking change on purpose

Removing a response property, adding a required request property, and the other changes that oasdiff rates as errors fail the check, with the rule of oasdiff and the line in the document.
When the change is intended, raise `info.version` of the document in the same pull request: below 1.0, raise the minor version (`0.1.0` to `0.2.0`); from 1.0 on, raise the major version (`1.4.0` to `2.0.0`).
oasdiff then accepts the breaking changes of that document, and the raised version shows the reviewer that the pull request breaks the contract on purpose.
The reviewer decides whether every client can follow, and the clients in this repository, such as the frontend and the generated client of another context, change in the same pull request.
The check compares with the merge base of `origin/main`, or of `CHECK_BASE` when it is set, as CI does. A new document has nothing to compare with, and passes.

## Follow the change in the clients

After a change to an API document, `make gen` regenerates the code on both sides of the contract: the Go server of the context and the clients of other contexts (oapi-codegen), and the TypeScript types of the frontend, `frontend/src/lib/api/<context>.gen.ts` (openapi-typescript).
The type check of the frontend then names the code of the screens that no longer fits the types: `lint-frontend-types` fails at each call whose path, parameters, or body the document no longer allows, and at each use of a part of an answer that is gone.
A breaking change updates those screens in the same pull request. `make e2e` shows whether the screens still work with the changed API.

## Check the API against requests generated from its document

`make check-all` runs `test-api-fuzz` (TPL-0007, layer 5), which looks at how the API answers requests that no test sends.
Schemathesis reads each document in `contracts/http/`, generates requests from it, and sends them to the API of the context.
The check builds the APIs from the checkout and runs them on a database of their own on the local PostgreSQL, so it needs Docker, but not `make up`, and it leaves the data of the local system alone.

For each operation, Schemathesis sends:

- the examples of the document;
- requests that test each constraint of the document from both sides: a value at a limit and one past it, a missing required property or header, a value of another type, a method that the path does not have, and a body in a media type that the operation does not take;
- 100 requests that it generates from the schemas, valid and invalid;
- sequences that follow the `Location` header of a new resource, such as registering a room and reading it.

An answer must have a status that the operation declares, or 405 with the header `Allow` for a method that the path does not have, and the content type, the headers, and the body that the document gives for that status.
A request that the document does not allow must get a 4xx, a request that it allows must not be refused as unreadable or invalid, and no request may get a 5xx.
The settings of the check are in `tools/schemathesis/schemathesis.toml`.

Run the check alone:

```sh
make test-api-fuzz
```

It takes about a minute, and up to two when the machine is busy with other work.
With the same code and the same documents, every run sends the same requests, so a finding comes back until its cause is fixed.

### Read a finding

A finding names the line of the operation in the document, and the check of Schemathesis that failed:

```text
contracts/http/facility.openapi.yaml:82: [schemathesis/status_code_conformance] GET /rooms/{roomId}: Undocumented status code (ST103): Received: 410; Documented: 200, 400, 404, 422, 500. Answer: 410 room-not-found: no room has the id 4a6ffa02-1a33-56f0-a26b-39316053eee8. Request: GET /rooms/4a6ffa02-1a33-56f0-a26b-39316053eee8
```

`Answer` gives the status, and for a problem its kind and its detail; `Request` gives what was sent, without the base URL.
`.cache/api-fuzz/<context>/output.txt` holds the report of Schemathesis, with a curl command for each finding, and `.cache/api-fuzz/<context>/api.log` holds the log of the API, with the error behind a 500.

To send the request again, start the local system with `make up`, and run the curl command of the finding with the base URL of the API: `http://127.0.0.1:18081` for Facility, `http://127.0.0.1:18082` for Reservation.
An id that the API made, such as the id of a new room, differs from run to run.

### Fix a finding

Decide which side is wrong, and fix that side:

- The document is wrong when the API gives an answer on purpose, such as a status or a header, and the document does not list it. Add the answer to the document; when the change breaks a client, follow [Make a breaking change on purpose](#make-a-breaking-change-on-purpose).
- The code is wrong when the API refuses a request that the document allows, gives an answer that the document does not list, or fails with a 500. Fix the code, and add an API test that sends the request (layer 4), so that `make check` catches it too.
- A rule that a JSON Schema cannot express, such as a period that ends after it starts, can refuse a request that the document allows, with a 4xx that the operation declares. Then neither side is wrong: add the status to `positive_data_acceptance` of the operation in `tools/schemathesis/schemathesis.toml`, with the rule as the reason.

A check is never turned off for every operation: a finding that the tool gets wrong is configured for its one operation, with the reason next to it.
