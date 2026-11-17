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
- Each consuming context has its own SQS queue, with a dead-letter queue. An EventBridge rule sends the events that the context needs to its queue. The rule lives in the consumer's infrastructure unit (TPL-0015), so the publisher does not know its consumers.
- Every event uses the CloudEvents 1.0 envelope. The payload of each type of event is defined by a JSON Schema in `contracts/events/<context>/<Event>.v1.json`. A change that would break consumers is a new version of the event, in a new file.
- An AsyncAPI 3.0 document describes the channels: which context publishes which events, and which contexts consume them.
- Consumers are idempotent: handling the same event twice has the effect of handling it once, for example because the consumer records the `id` of each event that it has handled.
- The domain and the use cases do not know EventBridge or SQS. They reach the outbox and the queues through ports and adapters (TPL-0004).
- The sample has two events, `ReservationConfirmed` and `ReservationCancelled`, which Reservation publishes and Notification consumes.

### Consequences

- Good, because an event is stored if and only if its state change commits, so no event is lost and none reports a change that rolled back.
- Good, because a new consumer is a new queue and a new rule in that consumer's unit. The publisher does not change.
- Good, because each consumer has its own queue and dead-letter queue. A slow or failing consumer does not affect the others, and a message that keeps failing is set aside after a fixed number of attempts.
- Good, because every part of the path except the relay is a managed service, and there is no broker to run.
- Bad, because delivery is at least once and the order of events is not guaranteed, so every consumer must be idempotent and must cope with events that arrive out of order.
- Bad, because the relay is one more part to build and run, and it adds a delay between the commit and the delivery that depends on how often it reads the outbox.
- Bad, because the outbox table grows, and published rows must be removed.
- Bad, because a test of the whole flow needs AWS or an emulator of it (TPL-0017).

### Confirmation

M0 has no events, so nothing enforces this decision yet.
The following checks arrive with the events milestone.

- The tests of a publisher validate each payload against the JSON Schema of its event.
- A compatibility check compares each changed JSON Schema with the version on the main branch, and fails on a change that would break consumers.
- Tests run the outbox, the relay, EventBridge, and SQS against the local emulator (TPL-0017), in CI.

Until then, review enforces the decision.

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

- Not decided yet: how the relay is implemented. The candidates are Watermill, River, and code of our own. A record decides this with the events milestone.
- If AWS Emulator turns out not to support the delivery from EventBridge to SQS, the fallback is that the relay reads a table of subscriptions and sends events to the SQS queues itself (TPL-0017).
- [CloudEvents 1.0](https://github.com/cloudevents/spec)
- [AsyncAPI 3.0](https://www.asyncapi.com/docs/reference/specification/v3.0.0)
- [The transactional outbox pattern](https://microservices.io/patterns/data/transactional-outbox.html)
