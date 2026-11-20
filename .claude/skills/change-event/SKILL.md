---
name: change-event
description: Change or add an integration event, its JSON Schema in contracts/events/<context>/, its AsyncAPI message, the mapping in the publisher, and what each consumer reads; or subscribe a context to events. Use when a change touches a domain event that other contexts see, or when lint-events or lint-events-breaking reports a finding.
---

# Change an event

Follow `docs/guides/event-changes.md`. It has the rules of `lint-events` and `lint-events-breaking`, when a change needs a new version, and the steps to add an event or to subscribe a consumer; this skill adds only what to decide and to verify.

## Steps

1. Decide first whether every consumer of the current version can still read the change. If not, the change is a new version in a new file, `<Name>.v<N+1>.json`, and the old file stays as it is at the base. Never edit or delete a schema file to make `lint-events-breaking` pass.
2. Change the JSON Schema before any Go code, then the AsyncAPI document of the publisher. Keep the object open, and give every property a `description` and the schema an example of real data.
3. In the publisher, map the domain event in `adapter/postgres/events.go`, and pass the event through `eventtest.AssertMatchesContract` in `events_test.go`. Never drop a domain event without an integration event.
4. In each consumer that handles the event, read its data in the anticorruption layer, keep the handler idempotent, and check that the test that reads every example of the schema covers the new type.
5. For a new consumer, follow "Subscribe to events" in the guide: its AsyncAPI document with `receive` operations, its subscription in `infra/local/main.tf`, its command, and its service in `compose.yaml`. No check reports a missing AsyncAPI document of a consumer, so verify it yourself.
6. Verify, before you finish:
   - the READMEs of the publisher and of each consumer in `docs/contexts/`, and their translations, list the event;
   - the subscription in `infra/local/main.tf` names the `type` of the event, if a consumer handles it.
7. Run `make fmt`, then `make check`. Run `make test-integration` too: it needs Docker, takes about a minute, and runs the tests that compare whole events, such as those of the relay. `CHECK_SCOPE=changed make check-all` adds `make smoke` and takes many minutes; start it last.
