---
name: add-context
description: Add a bounded context to the backend, with its API document, code, command, schema and roles, glossary, and README. Use when the business needs a new bounded context, such as Notification.
---

# Add a context

Follow `docs/guides/add-a-context.md`. It has the steps and a table of the check that reports each missing piece; this skill adds only what to decide and to verify.

## Steps

1. Before you write anything, state the name of the context (one lowercase word) and its kind of subdomain, core or supporting, with the reason, and confirm both with the person who asked. The kind decides whether you copy Reservation or Facility, and no check can judge it (TPL-0003).
2. Follow the guide step by step. Run `make gen` after the API document and the code are in place; never copy or edit `*.gen.go`.
3. Integrate with other contexts only through their HTTP APIs, with a generated client in `internal/adapter/<other>/`, or through events (TPL-0005). Never import their packages or read their schemas.
4. Verify, before you finish:
   - `db/bootstrap.sql` and `db/local.sql` have the roles, the schema, and the passwords of the context, and `make db-up` applies its schema;
   - the glossary, the README, and their translations exist, and `ARCHITECTURE.md` and its translation name the context;
   - the integration tests of the context connect as its runtime role.
5. Run `make fmt` and `CHECK_SCOPE=changed make check-all`, which needs Docker. Finish with `make check`.
