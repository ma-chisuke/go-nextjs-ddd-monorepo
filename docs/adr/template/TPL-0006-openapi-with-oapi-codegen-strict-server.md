---
status: accepted
date: 2026-11-17
---

# TPL-0006: Write each HTTP contract in OpenAPI 3.1, and generate the Go server with the oapi-codegen strict server

## Context and Problem Statement

Every bounded context offers an HTTP API, which the Next.js app and other contexts call (TPL-0005).
The Go server, the TypeScript code of the screens, and the clients between contexts must agree on every path, parameter, body, and status code.
When each side is written by hand, the sides drift apart: a field renamed in Go keeps its old name in TypeScript, and the mistake shows only at run time.

Two choices decide how the sides stay in agreement.
The first is which side is the source: a document that describes the API, or the code that implements it.
The second is which tools turn the source into the code on each side.

How is each API described, and how does the code on each side follow the description?

## Decision Drivers

- The contract is reviewed as a change of its own, before or together with the code that implements it.
- A handler that does not match the contract fails to compile.
- The server uses `net/http` from the standard library, with no web framework.
- The format is a published standard that the checks of TPL-0007 and the TypeScript tools can read.
- The API follows the semantics of HTTP: status codes, `Location`, `ETag` and `If-Match`, and problem details for errors (TPL-0008).

## Considered Options

- OpenAPI 3.1 written first, with the oapi-codegen strict server and openapi-typescript
- OpenAPI derived from the Go code or from annotations in it
- OpenAPI 3.1 written first, with ogen
- gRPC or Connect, with Protocol Buffers
- GraphQL

## Decision Outcome

Chosen option: "OpenAPI 3.1 written first, with the oapi-codegen strict server and openapi-typescript", because it makes the contract the reviewed source, turns a mismatch in the Go code into a compile error, and keeps the server on the standard library.

- `contracts/http/<context>.openapi.yaml` is the source of the API of each context. A change of the API starts with a change of this file.
- oapi-codegen generates the strict server interface and the types for the Go server, on `net/http`. The strict server gives each operation a typed request object and typed response objects, so a handler can return only a status and a body that the document declares.
- oapi-codegen also generates the client that another context uses to call the API (TPL-0005).
- openapi-typescript generates the TypeScript types that the Next.js app uses.
- Generated code is committed, in files named `*.gen.go` and `*.gen.ts`, which `.gitattributes` collapses in pull request diffs. `make gen` writes these files, and nobody edits them by hand.
- The descriptions in the documents are in English (TPL-0021).

Not yet verified: whether an OpenAPI 3.1 document passes through every tool in the chain (oapi-codegen, openapi-typescript, kin-openapi, Schemathesis, oasdiff, and Redocly CLI).
The first bounded context tests this.
If one of the tools cannot read 3.1, the documents use OpenAPI 3.0.3, and a new record states the change and the reason.

### Consequences

- Good, because a reviewer sees a change of the API as a change of one YAML file, apart from the code.
- Good, because a missing handler, or a handler with a wrong parameter or response type, fails to compile.
- Good, because the Go server, the TypeScript types, and the clients between contexts all come from the same file, so they agree with each other after each run of `make gen`.
- Good, because many tools read OpenAPI, including the ones that check the agreement between the document and the code (TPL-0007).
- Bad, because generated code lives in the repository, makes the repository larger, and must be regenerated after every change of a document.
- Bad, because the generators limit what a document can use. Some schema constructs, such as `oneOf`, become Go types that are awkward to use, so the documents are written to suit the generators.
- Bad, because the generated code answers some errors by itself, such as a parameter that cannot be parsed, and the template must replace those answers so that every error is a problem (TPL-0008).
- Bad, because people and agents write the document before the code, which is slower for a change that is only a few lines long.

### Confirmation

No check of this decision exists yet: the repository has no OpenAPI document and no generator.
These checks arrive with the first bounded context (milestone M1).

- The compiler: the generated code takes an implementation of the strict server interface, so a missing or wrongly typed handler is a compile error. `lint-go` and `test-go` report it.
- Freshness: a check runs `make gen` and fails when the generated files differ from the committed ones.
- Routes: a lint rule forbids registering an HTTP route by hand, so every route comes from the generated code.

TPL-0007 describes these checks, and the ones that catch what types cannot express.

## Pros and Cons of the Options

### OpenAPI 3.1 written first, with the oapi-codegen strict server and openapi-typescript

- Good, because the strict server turns the document into a Go interface, and the compiler checks the implementation against it.
- Good, because the server stays on `net/http`, and middleware is ordinary `net/http` middleware.
- Bad, because the generated code does not check the constraints that types cannot express, such as patterns and ranges. Another layer must check them (TPL-0007).

### OpenAPI derived from the Go code or from annotations in it

- Good, because there is one source, and no generated server code.
- Bad, because the document follows the code: a change of the API becomes visible only after the code is written, and the screens wait for the server.
- Bad, because a change to a Go struct can change the API without anyone meaning to, and only review would notice.

### OpenAPI 3.1 written first, with ogen

- Good, because ogen also generates code that checks constraints from the document, such as lengths and patterns.
- Bad, because the generated server routes requests with its own router instead of `net/http`, and wraps optional fields in types of its own.
- Bad, because fewer projects use it than oapi-codegen, so people and agents find fewer examples to follow.

### gRPC or Connect, with Protocol Buffers

- Good, because Protocol Buffers give a compact, typed contract, and tools such as Buf detect breaking changes.
- Good, because Connect also serves plain HTTP with JSON, which a browser can call.
- Bad, because the API would not follow the semantics of HTTP: status codes, conditional requests with `ETag`, and problem details do not map onto RPC errors.
- Bad, because the checks of TPL-0007, such as Schemathesis and oasdiff, read OpenAPI, not Protocol Buffers.

### GraphQL

- Good, because a screen asks for exactly the fields that it needs, from several types in one request.
- Bad, because errors usually arrive in the body of a response with status 200, so status codes and problem details (TPL-0008) do not apply.
- Bad, because the APIs of the contexts would have to be merged into one graph for the screens, which adds a gateway and ties the schemas of the contexts together.

## More Information

- [OpenAPI Specification 3.1.0](https://spec.openapis.org/oas/v3.1.0)
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)
- [openapi-typescript](https://openapi-ts.dev/)
