---
status: accepted
date: 2026-11-17
---

# TPL-0017: Emulate AWS in the local environment with AWS Emulator

## Context and Problem Statement

The template must run on one machine with Docker Compose alone, without an AWS account.
A person who has just copied it, a coding agent in a sandbox, and CI all run the whole system the same way.

Most of the system is easy to run locally: PostgreSQL 18, each context, Next.js, and Mailpit to catch the mail that Notification sends.
AWS is the hard part.
Events travel through EventBridge and SQS (TPL-0013), and secrets come from Secrets Manager.
The code reaches AWS only through adapters (TPL-0004), but those adapters, and the routing from EventBridge rules to queues, need something to run against.

The usual answer was LocalStack.
Since March 2026, LocalStack has required an auth token, which needs an account, and a template that must run without an account cannot depend on it.
On November 11, 2026, AWS released AWS Emulator, its own emulator of AWS services.

How does the local environment provide the AWS services that the template uses?

## Decision Drivers

- The local environment runs with Docker Compose alone, with no account and no token.
- It covers the behaviors that the template relies on: EventBridge rules that deliver to SQS, dead-letter queues, and Secrets Manager.
- Ideally, the same Terraform definitions create the queues and rules locally and in AWS, so the two cannot drift apart.
- The license does not burden a team that copies the template.

## Considered Options

- AWS Emulator
- LocalStack
- A real AWS account for each developer
- A separate fake for each service
- No local AWS at all, with the ports replaced by fakes in tests

## Decision Outcome

Chosen option: "AWS Emulator", because it is the only option that provides the AWS services of the template locally without an account.

The rules are as follows.

- Docker Compose runs the whole system: PostgreSQL 18, AWS Emulator, each context, Next.js, and Mailpit for mail.
- AWS publishes AWS Emulator as a Linux package and a Dockerfile, but not as a container image, so the template builds an image from the published Dockerfile, at a pinned version.
- AWS Emulator sends telemetry by default. The template sets `EMULATOR_TELEMETRY=off`.
- AWS Emulator keeps its data in memory only, so the local environment creates its queues, rules, and secrets each time it starts.
- The list of supported services was not published when this decision was made. The behaviors that the template relies on (EventBridge to SQS, dead-letter queues, and Secrets Manager) each get a small test that runs in CI.
- If those tests fail, the fallback is that the relay of the outbox reads a table of subscriptions and sends events to the SQS queues itself, without EventBridge (TPL-0013).
- If Terraform can be applied to AWS Emulator, the local environment applies the same Terraform definitions to it, so the queues and rules come from the same code as in AWS (TPL-0015).

### Consequences

- Good, because anyone can run the whole system, including the flow of events, without an account or a token, and CI runs it the same way.
- Good, because AWS Emulator comes from AWS itself and is released under CC0, so it puts no license terms on a team that copies the template.
- Good, because, if Terraform applies to it, local queues and rules come from the definitions that AWS uses.
- Bad, because the product was released on November 11, 2026, and is unproven. Its coverage of the services and behaviors that the template needs is unknown until the tests run, and it may change quickly.
- Bad, because its state is lost when it restarts, so the local environment must recreate the AWS resources every time.
- Bad, because the image is built locally from the Dockerfile, which adds a step and some build time to the first start on every machine.
- Bad, because an emulator is not AWS. Permissions, quotas, and latency can differ, so stg remains the first place where the real services run.

### Confirmation

M0 has no local environment yet, so nothing enforces this decision yet.
With the events milestone, the tests of AWS Emulator (EventBridge to SQS, dead-letter queues, and Secrets Manager) run in CI. They fail when a version of the emulator does not support a behavior that the template relies on.
Until then, review enforces the decision.

## Pros and Cons of the Options

### AWS Emulator

- Good, because it comes from AWS, is released under CC0, and needs no account.
- Bad, because it is new and its coverage is unknown, it keeps data in memory only, the image is built locally, and telemetry is on by default.

### LocalStack

- Good, because it is mature, widely used, and covers many services, with tools for Terraform.
- Bad, because since March 2026 it has required an auth token, which needs an account, and the template must run without one.

### A real AWS account for each developer

- Good, because it is the real service, with no gaps in behavior.
- Bad, because every person, agent, and CI run needs an account, credentials, and a budget, and must clean up after itself. This breaks the rule that the template runs without an account.
- Bad, because it needs a network connection, and creating the resources for each run is slow.

### A separate fake for each service

- Good, because some fakes are small and mature, for example ElasticMQ, which implements the API of SQS.
- Bad, because each service needs its own fake, and the routing from EventBridge rules to SQS queues, which crosses services, is the behavior that the template most needs to test.

### No local AWS at all, with the ports replaced by fakes in tests

- Good, because there is nothing to run: tests replace the ports with fakes written in Go (TPL-0004).
- Bad, because the adapters and the routing between EventBridge and SQS never run before stg, so their bugs appear late.

## More Information

Facts about AWS Emulator, as of November 2026, from [the announcement](https://aws.amazon.com/about-aws/whats-new/2026/11/aws-emulator-now-available/):

- AWS released it on November 11, 2026.
- It is released under CC0, needs no account, and keeps its data in memory only.
- AWS publishes a Linux package and a Dockerfile, but no container image.
- It sends telemetry by default; `EMULATOR_TELEMETRY=off` turns it off.
- The list of supported services was not published.

Not yet verified: delivery from EventBridge to SQS, dead-letter queues, Secrets Manager, and applying Terraform to AWS Emulator. These are checked with the events milestone.
