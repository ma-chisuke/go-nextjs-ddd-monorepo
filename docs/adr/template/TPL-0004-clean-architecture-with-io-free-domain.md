---
status: accepted
date: 2026-11-17
---

# TPL-0004: Use a clean architecture, with a domain layer that does no I/O

## Context and Problem Statement

A core bounded context (TPL-0003) holds the rules that the product exists for.
Those rules change for business reasons, and they must be easy to read and to test.

When business rules call the database, read the clock, or write logs, they are hard to test and hard to read.
A test of one rule then needs a database, a way to fix the clock, and a logger, and a reader must separate the rule from the plumbing around it.
A change of technology, such as a new database driver or a new HTTP library, also reaches the rules.

Time and randomness are the subtle cases.
A rule that calls `time.Now` gives a different answer on each run, and a test can fix the answer only by replacing something global.
The same holds for a rule that creates its own random IDs.

Inside a core context, which code may depend on which, and where does I/O happen?

## Decision Drivers

- A business rule can be tested as a plain function call: no database, no network, no fake clock.
- The domain code uses the words of the business, not the words of the transport or of the storage.
- Dependencies point in one direction, and a program can check the direction.
- The layout is ordinary Go: small interfaces, declared by the code that uses them.

## Considered Options

- Clean architecture, with the ports in the app layer and no I/O in the domain
- Clean architecture, with the ports in the domain layer
- A layered architecture whose domain objects depend on persistence
- No layers

## Decision Outcome

Chosen option: "Clean architecture, with the ports in the app layer and no I/O in the domain", because it is the only option whose domain code needs nothing from outside to run, and because Go code usually declares an interface next to the code that uses it.

A core context has this layout:

```text
backend/internal/<context>/
├── module.go         wires the layers together; only cmd/<context> uses it
└── internal/
    ├── domain/       aggregates, value objects, domain events, business errors
    ├── app/          use cases (one file per business operation) and ports
    └── adapter/
        ├── http/     implementation of the generated strict server (TPL-0006)
        ├── postgres/ schema, queries, and repositories (TPL-0010, TPL-0011)
        └── event/    writes to the outbox, and receives other contexts' events (TPL-0013)
```

- Dependencies point inward: `adapter` imports `app` and `domain`, `app` imports `domain`, and `domain` imports neither.
- The domain does not use `context.Context`, the clock (`time.Now`), random numbers, the database, HTTP, the file system, or logging. It takes values, and it returns values, domain events, and business errors.
- A use case gets the current time and new IDs from ports, and passes them to the domain as arguments.
- Ports are interfaces in `app`: repositories, the clock, the ID generator, and the clients of other bounded contexts (TPL-0005). The adapters implement them.
- `module.go` builds the adapters, the use cases, and the HTTP handler. Only `cmd/<context>` imports it.
- A supporting context has no `domain` layer: its use cases call the generated queries directly (TPL-0003).

### Consequences

- Good, because a domain test builds values, calls a function, and compares the result. It needs no mocks.
- Good, because a rule that depends on time takes the time as an argument, so each test states the moment that it checks.
- Good, because the database, the HTTP library, and the message broker can change without a change to the domain.
- Bad, because data is mapped at each boundary: from the generated HTTP types to the domain, and from the domain to the rows that sqlc reads and writes. The mapping is code to write and to keep in step.
- Bad, because the current time and new IDs travel as parameters through each use case, which makes the signatures longer.
- Bad, because the domain cannot load data when it needs it. A rule that needs data from outside its aggregate gets that data from the use case, which loads it first.
- Bad, because supporting contexts use a different layout, so the layout of one context does not predict that of the next (TPL-0003).

### Confirmation

No check of the layers exists yet; they arrive with the first bounded context (milestone M1).
`.golangci.yml` already turns on depguard and forbidigo, but only with rules that apply to every Go file, and it states that the rules for the layers come with the first bounded context.

The checks that arrive then are as follows.

- Import rules, with depguard or go-arch-lint (the choice is made when the rules are written): the domain imports only its own packages and an allow-list of standard library packages, which leaves out `context`, `database/sql`, `net/http`, `os`, `log/slog`, and the random number packages; `app` does not import `adapter`; only `cmd/<context>` imports the package of `module.go`.
- forbidigo: the domain does not call functions that read the clock, such as `time.Now` and `time.Since`. The `time` package stays on the allow-list for the type `time.Time`, so the import rules alone cannot catch these calls.
- Go `internal` directories: the compiler refuses an import of `backend/internal/<context>/internal/...` from outside `backend/internal/<context>/`.

## Pros and Cons of the Options

### Clean architecture, with the ports in the app layer and no I/O in the domain

- Good, because the domain has no interfaces to fake, so its tests are plain function calls.
- Good, because each port sits next to the use cases that call it.
- Bad, because readers who know the DDD layouts of Java or C# expect the repository interfaces in the domain, and look for them there first.

### Clean architecture, with the ports in the domain layer

- Good, because it matches the layout of many DDD books, where the domain declares the repository of each aggregate.
- Bad, because a repository method takes a `context.Context` and returns errors from storage, so the domain would carry I/O in its signatures even if it never called them.
- Bad, because domain code could then call a repository, and the rule that the domain does no I/O would rest on review alone.

### A layered architecture whose domain objects depend on persistence

- Good, because there is less code: an active record or an ORM entity holds the rules, loads and saves itself, and needs no mapping.
- Bad, because a test of a rule needs a database, or a fake of the persistence library.
- Bad, because the shape of the tables sets the shape of the model, and a change to either reaches the other.

### No layers

- Good, because a handler that reads the request, applies the rule, and writes the row is the shortest code.
- Bad, because the rules are spread across handlers, mixed with parsing and SQL, and can be tested only through HTTP.
- Bad, because no program can check the direction of dependencies in code that has no layers.

## More Information

- [The Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html), by Robert C. Martin
- "Architectural Patterns", in Vlad Khononov, "Learning Domain-Driven Design" (O'Reilly, 2021): layered architecture, and ports and adapters.
- [Internal packages](https://go.dev/doc/go1.4#internalpackages), in the release notes of Go 1.4, which introduced them
