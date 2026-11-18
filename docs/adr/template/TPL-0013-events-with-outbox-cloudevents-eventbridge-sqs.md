---
status: accepted
date: 2026-11-17
---

# TPL-0013: Publish integration events through an outbox to EventBridge, as CloudEvents, with an SQS queue for each consuming context

## Context and Problem Statement

Contexts work together through APIs and events (TPL-0005).
In the sample, Reservation tells Notification that a reservation was confirmed or cancelled, and Notification sends mail.

Publishing an event looks simple, but it is not.
A context changes its state in its database and must also hand a message to a broker. These are two systems, and no transaction covers both.
If the service commits and then crashes before it sends, the event is lost. If it sends first and the transaction then fails, other contexts act on a change that never happened.

Consumers have problems of their own.
A consumer can be down when an event is sent, can fail on one message again and again, and can receive the same message twice.

The shape of an event matters as well.
An integration event crosses the boundary of a context, so its shape is a published contract, like an API (TPL-0006). Consumers depend on it, and it changes on its own schedule.

How do contexts publish and receive integration events reliably on AWS, and how is the shape of an event defined?

## Decision Drivers

- An event is published if and only if the state change that it reports is committed.
- The publisher does not know its consumers. Adding a consumer changes nothing in the publisher.
- A consumer that is down or failing loses no events, and one bad message does not hold up the rest.
- The shape of each event is a contract that a machine can check.
- Managed AWS services, with no broker cluster to run, and a way to run the whole flow locally (TPL-0017).

## Considered Options

- An outbox and a relay, EventBridge, and an SQS queue for each consuming context
- Publishing to the broker right after the commit (dual write)
- SNS topics with SQS queues
- Kafka (Amazon MSK)
- Change data capture from the database

## Decision Outcome

Chosen option: "An outbox and a relay, EventBridge, and an SQS queue for each consuming context", because the outbox makes publishing as reliable as the commit, and EventBridge with a queue for each consumer lets consumers subscribe without the publisher knowing them, on managed services.

The rules are as follows.

- A context writes each integration event to an outbox table in its own schema, in the same transaction as the state change.
- A relay reads new rows from the outbox, publishes them to a dedicated EventBridge event bus, and then marks them as published. If the relay fails between the two steps, it publishes the row again, so delivery is at least once.
- The relay is code of this repository, `backend/internal/platform/outbox`, and runs as a process of its own, from the image of its context (TPL-0014).
- On the bus, an event is an EventBridge entry whose `source` and `detail-type` are the `source` and the `type` of its envelope, and whose `detail` is the envelope itself.
- Each consuming context has its own SQS queue, with a dead-letter queue. An EventBridge rule selects the events that the context needs by `source` and `detail-type`, and sends the `detail` alone to the queue (the input path `$.detail`), so the queue receives the envelope as the publisher wrote it. The rule lives in the consumer's infrastructure unit (TPL-0015), so the publisher does not know its consumers.
- Every event uses the CloudEvents 1.0 envelope. The payload of each type of event is defined by a JSON Schema in `contracts/events/<context>/<Event>.v1.json`. A change that would break consumers is a new version of the event, in a new file.
- An AsyncAPI 3.0 document describes the channels: which context publishes which events, and which contexts consume them.
- Consumers are idempotent: handling the same event twice has the effect of handling it once, for example because the consumer records the `id` of each event that it has handled.
- The domain and the use cases do not know EventBridge or SQS. They reach the outbox and the queues through ports and adapters (TPL-0004).
- The sample has two events, `ReservationConfirmed` and `ReservationCancelled`, which Reservation publishes and Notification consumes.

The relay is code of our own, not a library, because what it does is small and the libraries bring more than that.

- River is a job queue for Go and PostgreSQL. It owns its tables and their migrations, while each context declares its tables in its own `schema.sql` (TPL-0010) and owns one schema (TPL-0009).
- Watermill offers an outbox for SQL, with a table layout and a message abstraction of its own, and has no publisher for EventBridge, so an adapter would be needed anyway.
- The relay reads the waiting rows in order, publishes each, and marks it. About 170 lines do that, behind two ports: `Source`, which a context implements over its outbox, and `Publisher`, which `platform/awsevents` implements over EventBridge. The outbox is a table of the `schema.sql` of its context, its queries go through sqlc (TPL-0011), and the tests of the relay are plain Go tests with fakes of the two ports (`outbox/outboxtest`).

The relay looks at the outbox once a second. It takes up to 20 waiting rows in the order of their ids with `FOR UPDATE SKIP LOCKED`, so several relays can run at once and each event goes to one of them. It publishes each event with a call of its own to `PutEvents`, and marks the published rows in the same transaction. It deletes a published row after seven days (`outbox.Config`).

Two parts of this decision differ from the first plan.

- The first plan left the relay open, with Watermill, River, and code of our own as candidates, for the events milestone to decide.
- The first plan had a fallback in case AWS Emulator could not deliver from EventBridge to SQS: the relay would read a table of subscriptions and send the events to the queues itself. AWS Emulator delivers them (TPL-0017), so the fallback was not built.

### Consequences

- Good, because an event is stored if and only if its state change commits, so no event is lost and none reports a change that rolled back.
- Good, because a new consumer is a new queue and a new rule in that consumer's unit. The publisher does not change.
- Good, because each consumer has its own queue and dead-letter queue. A slow or failing consumer does not affect the others, and a message that keeps failing is set aside after a fixed number of attempts.
- Good, because every part of the path except the relay is a managed service, and there is no broker to run.
- Bad, because delivery is at least once and the order of events is not guaranteed, so every consumer must be idempotent and must cope with events that arrive out of order.
- Bad, because the relay is one more part to run, and it adds a delay of up to one second, its interval, between the commit and the publishing.
- Bad, because the relay is code that this repository maintains. It publishes one event per call, while `PutEvents` takes up to ten.
- Bad, because an event that the broker always refuses, and a row of the outbox that cannot be read, stay in the outbox. The relay tries them again at every interval and logs each failure, with no count of attempts and no place to set them aside, so a person must find them in the log.
- Bad, because the outbox table grows. The relay keeps published rows for seven days, and then deletes them.
- Bad, because a test of the whole flow needs AWS or an emulator of it (TPL-0017).

### Confirmation

These checks enforce the decision.

- `lint-events`, in `make check`, checks the JSON Schema of each event: its file name, its form, an object that stays open, examples that match it, and a message of the AsyncAPI document of its context that refers to it. [Change an event](../../guides/event-changes.md) lists the rules.
- `lint-events-breaking`, in `make check`, compares each JSON Schema with its version at the merge base with `origin/main`, and fails on a change that a consumer of that version could not read, and on a deleted schema file.
- `lint-contracts` lints the AsyncAPI documents with Redocly CLI.
- The tests of the publisher pass every integration event that Reservation makes through `eventtest.AssertMatchesContract`, which validates it against the JSON Schema of its type (`adapter/postgres/events_test.go`). They also fail when the domain gains a type of event that has no integration event.
- The tests of the consumer turn every example of the JSON Schemas that Notification reads into a mail (`internal/app/events_test.go` of Notification), so an example that it cannot read fails them.
- The integration tests of Reservation check that the store writes the events of a change with the change and none with a refused change, that two relays never take the same event, and that a row that cannot be read does not hold up the others.
- The integration tests of Notification deliver one event twice, also twice at the same moment, and expect one mail.
- The integration tests of `platform/awsevents` run the publisher and the consumer against AWS Emulator: an event reaches the handler of a subscriber through a rule and a queue, and an event whose handler keeps failing moves to the dead-letter queue. They run in `make test-integration`, part of `make check-all` and of CI.
- `make smoke` books, reschedules, and cancels a reservation in the local system, and waits for the three mails in Mailpit, through the outbox, the relay, the bus, the queue, and the worker of Notification.

## Pros and Cons of the Options

### An outbox and a relay, EventBridge, and an SQS queue for each consuming context

- Good, because publishing succeeds or fails with the commit, consumers are decoupled from the publisher and isolated from one another, and the services are managed.
- Good, because EventBridge rules can match on any field of an event, each consumer owns its rules, and EventBridge can archive events and replay them.
- Bad, because the relay must be built and run, delivery is at least once, and the order is not guaranteed.

### Publishing to the broker right after the commit (dual write)

- Good, because it is the simplest option: no outbox table and no relay.
- Bad, because the commit and the publish are two systems without a shared transaction. A crash between them loses the event, and publishing before the commit can report a change that then rolls back.

### SNS topics with SQS queues

- Good, because SNS with a queue for each subscriber is a mature and cheap way to fan out messages, with filter policies to select them.
- Good, because SNS charges less per message than EventBridge and adds less delay.
- Bad, because the publisher must create and name a topic for each group of events, and consumers must know those topics, while EventBridge has one bus whose rules can match on any field.

### Kafka (Amazon MSK)

- Good, because it keeps an ordered log for each partition, consumers can replay it from any point, and it handles very high throughput.
- Bad, because a cluster costs money even when idle and needs knowledge of partitions, consumer groups, and retention, far beyond what the sample needs.
- Bad, because the outbox is still needed: a write to Kafka does not join the database transaction either.

### Change data capture from the database

- Good, because the log of the database is the source of events, so there is no dual write and no polling of an outbox.
- Bad, because a pipeline for change data capture, for example Debezium with Kafka Connect, is a large system to run.
- Bad, because capturing table changes publishes the internal tables of a context, unless it is combined with an outbox. On Aurora, it also needs logical replication, which keeps a Serverless v2 cluster from pausing (TPL-0016).

## More Information

- Not decided yet: how a version of an event that no consumer reads any more is retired. `lint-events-breaking` fails when a schema file is deleted, so every version stays for now.
- [Change an event](../../guides/event-changes.md) describes how to change or add an event, and how a new consumer subscribes.
- [CloudEvents 1.0](https://github.com/cloudevents/spec)
- [AsyncAPI 3.0](https://www.asyncapi.com/docs/reference/specification/v3.0.0)
- [The transactional outbox pattern](https://microservices.io/patterns/data/transactional-outbox.html)
