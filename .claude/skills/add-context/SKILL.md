---
name: add-context
description: Add a bounded context to the backend, with its contracts, code, command, schema and roles, glossary, and README; an API, a worker that consumes events, or both. Use when the business needs a new bounded context, such as Notification.
---

# Add a context

Follow `docs/guides/add-a-context.md`. It has the steps and a table of the check that reports each missing piece; this skill adds only what to decide and to verify.

## Steps

1. Before you write anything, state the name of the context (one lowercase word), its kind of subdomain, core or supporting, with the reason, and whether it answers requests, consumes events as a worker, or both. Confirm them with the person who asked. They decide whether you copy Reservation, Facility, or Notification, and no check can judge them (TPL-0003).
2. Follow the guide step by step. Run `make gen` after the contracts and the code are in place; never copy or edit `*.gen.go`.
3. Integrate with other contexts only through their HTTP APIs, with a generated client in `internal/adapter/<other>/`, or through events (TPL-0005). Never import their packages or read their schemas.
4. For a worker, write its AsyncAPI document with a `receive` operation for each event, read the data of each event in its anticorruption layer, and make its handler idempotent: it records the id of each event under a unique constraint, in the transaction of its effect (TPL-0013). Subscribe it as `docs/guides/event-changes.md` describes, and add its service to `compose.yaml`.
5. Verify, before you finish:
   - `db/bootstrap.sql` and `db/local.sql` have the roles, the schema, and the passwords of the context, and `make db-up` applies its schema;
   - the glossary, the README, and their translations exist, and `ARCHITECTURE.md` and its translation name the context and its processes;
   - the integration tests of the context connect as its runtime role;
   - for a worker, its AsyncAPI document exists, its tests turn every example of the schemas that it reads into what it does, and `infra/local/main.tf` subscribes it. No check reports a missing one.
6. Run `make fmt` and `CHECK_SCOPE=changed make check-all`, which needs Docker. Finish with `make check`.
