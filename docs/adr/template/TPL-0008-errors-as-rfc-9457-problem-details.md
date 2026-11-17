---
status: accepted
date: 2026-11-17
---

# TPL-0008: Return every error as RFC 9457 problem details

## Context and Problem Statement

Every API returns errors: a request that fails validation, a booking that breaks a business rule, a resource that does not exist, and failures that nobody expected.
A client must be able to tell these errors apart in code, show a useful message, and point at the field that was wrong.

If each endpoint chooses its own error body, every client learns several shapes, and the OpenAPI documents cannot describe the errors with one schema.
If the server answers an unexpected failure with the text of the error, the answer can leak internal details, such as SQL, paths, or the names of other services.

Business errors raise a second problem.
The domain decides that a booking overlaps another one, or that it exceeds the capacity of the room (TPL-0004).
The HTTP adapter must turn each of these errors into a status and a body, and nothing makes sure that a new business error gets a precise answer instead of a generic 500.

Which format do errors use, and how does each business error get a precise answer?

## Decision Drivers

- One format for every error of every context, described once and used in every OpenAPI document.
- A published standard, so that clients and tools already know the format.
- Each kind of error has a stable identifier and a page that explains it.
- A validation error names each field that is wrong.
- An unexpected failure reveals nothing internal.

## Considered Options

- RFC 9457 problem details
- An error envelope of our own
- RFC 7807 problem details
- JSON:API error objects

## Decision Outcome

Chosen option: "RFC 9457 problem details", because it is the current standard for errors in HTTP APIs, it fits the semantics of HTTP that the API follows (TPL-0006), and it allows the extension that the template needs for validation errors.

- Every 4xx and 5xx response has the media type `application/problem+json`.
- The body has the members `type`, `title`, `status`, `detail`, and `instance`.
- `type` is a URI that resolves to a page under `docs/problems/`. The page explains the problem and what a client can do about it. A client tells problems apart by `type`, so a published URI never changes.
- A validation error also has an `errors` array. Each item has a `pointer`, a JSON Pointer (RFC 6901) to the wrong field, and a `detail` that says what is wrong with it, following the example in RFC 9457.
- Business errors are typed errors in the domain (TPL-0004). The HTTP adapter of each context holds one table that maps each business error to a problem: its `type` and its status.
- An error that is not in the table becomes a problem with status 500, and a generic `title` and `detail`. The real error goes to the log, never into the response.
- The generated code answers some errors by itself, such as a parameter that cannot be parsed (TPL-0006). The template replaces those handlers, so that they answer with a problem too.

### Consequences

- Good, because a client handles every error of every context with one parser, and tells the errors apart by `type`.
- Good, because each `type` leads to a page that explains the error, for the people and the agents who write clients.
- Good, because a new business error without an entry in the table fails a test, instead of turning into a 500 for a user.
- Good, because an unexpected failure gives the client a 500 and nothing more, while the details go to the log.
- Bad, because each `type` URI needs a page that stays at the same address. A team that copies the template points the URIs at its own site, and each page also needs a Japanese translation (TPL-0021).
- Bad, because making oapi-codegen produce responses with the media type `application/problem+json` needs care. This is not verified yet; the first endpoint tests it.
- Bad, because the standard leaves the shape of extension members open, so the template defines `errors` itself, and a client learns that part from the template's documents.

### Confirmation

No check of this decision exists yet: the repository has no OpenAPI document and no Go code.
These checks arrive with the first bounded context (milestone M1).

- A Redocly CLI rule fails when a 4xx or 5xx response in an OpenAPI document has a media type other than `application/problem+json` (TPL-0007, layer 7).
- A test in the HTTP adapter of each context fails when a business error of the domain has no entry in the table.
- The tests validate every response against the document (TPL-0007, layer 4), so an error body that is not a problem fails the test that produced it.

## Pros and Cons of the Options

### RFC 9457 problem details

- Good, because it is an IETF standard, and many HTTP frameworks and clients already produce and read it.
- Good, because it allows extension members such as `errors`, so the format can grow without leaving the standard.
- Bad, because clients that know only their own error format must learn this one.

### An error envelope of our own

- Good, because it can hold exactly what the template wants, with no standard to follow.
- Bad, because every client and every tool learns a shape that only this API uses.
- Bad, because it has no media type of its own, so a client cannot recognize an error by its media type.

### RFC 7807 problem details

- Good, because the format is the same as in RFC 9457, and some older libraries name this RFC.
- Bad, because RFC 9457 obsoletes it. RFC 9457 also adds a registry of common problem types and guidance for reporting several problems at once.

### JSON:API error objects

- Good, because its error object has `source.pointer` for the wrong field, close to what the template needs.
- Bad, because it belongs to the JSON:API specification, which has its own media type and rules for the whole API, and this API does not follow them.

## More Information

- [RFC 9457: Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457.html)
- [RFC 6901: JavaScript Object Notation (JSON) Pointer](https://www.rfc-editor.org/rfc/rfc6901.html)
