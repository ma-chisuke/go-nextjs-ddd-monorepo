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
- AWS publishes AWS Emulator as RPM and deb packages, and no container image. The template builds the image from a Dockerfile of its own, `infra/local/aws-emulator/Dockerfile`: on Amazon Linux 2023 minimal, pinned by digest, it installs the RPM package of a pinned version. Before it installs the package, it checks the SHA-256 of the package for each architecture, and `rpm` checks the signature of the package against the public key of AWS, committed as `signing-key.asc` (TPL-0024).
- AWS Emulator sends telemetry by default. The image sets `EMULATOR_TELEMETRY=off`.
- AWS Emulator keeps its data in memory only, so the local environment creates its event bus, queues, and rules each time the emulator starts.
- The local environment applies the Terraform modules that AWS will use, `infra/modules/event-bus` and `infra/modules/event-subscription`, to the emulator, so the local bus, queues, and rules come from the same code as in AWS (TPL-0015). The root that applies them, `infra/local/`, accepts only an endpoint on this machine. `tools/scripts/aws.sh apply` (`make aws-apply`, which `make up`, `make smoke`, and `make e2e` run) keeps the state of Terraform outside the repository, and throws it away when the emulator has restarted.
- The emulator serves HTTPS on port 9297. It makes its certificate at every start, for the names `localhost` and `127.0.0.1` only, and signs it with a CA that it writes to its state directory. The host gets that CA from `tools/scripts/aws.sh server` (`.cache/aws-emulator/ca.pem`). The containers of the services mount the volume that holds it, and `AWS_ENDPOINT_SERVER_NAME=localhost` tells them the name that the certificate carries, which they check rather than skip (`platform/awsconfig`, which refuses the variable without `AWS_ENDPOINT_URL`). The emulator runs as the user 65532, as the services do, because it makes the CA file readable by its owner only.
- The emulator checks no signature of requests (`--auth off`), because it makes new credentials at every start. Its port is published on the loopback address only.
- Every service of Docker Compose publishes its port on the loopback address only, and below 32768: PostgreSQL on 15432, AWS Emulator on 19297, Mailpit on 18025, Facility on 18081, Reservation on 18082, and the frontend on 13000. From 32768 up, Linux and macOS hand out ports by themselves, to outgoing connections too. A test that held a connection to the local PostgreSQL could sit on a port that a service was about to publish, and the service failed to start; this failed CI twice in one day, on two different ports, while the ports lay between 55432 and 59297.
- When `make up` starts the emulator anew, it restarts the services that mount the volume of its CA, so that they read the new one. A restart of the emulator in any other way leaves them to a person ([Run the services locally](../../guides/run-locally.md)).
- The behaviors that the template relies on each have a test that runs in CI, and the tests fail when a version of the emulator stops doing them (Confirmation).
- Nothing in the local system calls Secrets Manager or SES. The services read their secrets from environment variables, which ECS fills in AWS (TPL-0014), and Notification sends its mail to Mailpit. So the local environment creates no secrets, and no test of Secrets Manager or SES is kept.

Three parts of this decision differ from the first plan.

- The first plan was to build the image from the Dockerfile that AWS publishes. The template writes its own, so that the base image is pinned by digest and the package is checked before it is installed (TPL-0024).
- The first plan had a fallback in case the emulator could not deliver from EventBridge to SQS. It delivers, so the fallback was not built (TPL-0013).
- The first plan named Secrets Manager among the behaviors to test. It works in the emulator, but nothing in the local system uses it, so no test is kept for it.

### Consequences

- Good, because anyone can run the whole system, including the flow of events, without an account or a token, and CI runs it the same way.
- Good, because AWS Emulator comes from AWS itself and is released under CC0, so it puts no license terms on a team that copies the template.
- Good, because the local bus, queues, and rules come from the modules that AWS will use.
- Bad, because the product was released on November 11, 2026, and may change quickly. The tests cover only the behaviors that the template relies on, and one difference from AWS is known already: the emulator accepts an event for a bus that does not exist, and drops it. The relay therefore checks at start that its bus exists (`Publisher.Ready`, TPL-0013).
- Bad, because its state is lost when it restarts, so the local environment must recreate the AWS resources every time. The first apply after a start takes about a minute, because the AWS provider waits about 25 seconds after it creates each SQS queue, and a subscription has two. The module `event-subscription` sets the policy of the queue as an argument of the queue, because a separate policy resource would wait once more.
- Bad, because the CA changes at every start of the emulator. After the emulator restarts, the services that call it must restart too, and the host must copy the CA again. `make up` does both when it started the emulator anew; after a restart that `make up` did not cause, a person does them.
- Bad, because the image is built locally from the Dockerfile, which adds a step and some build time to the first start on every machine.
- Bad, because no Renovate rule updates the version of the emulator yet. An update changes the version and the two checksums in the Dockerfile by hand (TPL-0025).
- Bad, because an emulator is not AWS. Permissions, quotas, and latency can differ, so stg remains the first place where the real services run.

### Confirmation

Docker Compose runs PostgreSQL 18, AWS Emulator, Mailpit, the processes of each context, and Next.js. The processes are the APIs of Facility and Reservation, the relay of Reservation, and the worker of Notification; Next.js is the service `web`, built from `frontend/Dockerfile`, at <http://127.0.0.1:13000>. `make up` starts them.
Every process of the system runs in Compose. Only the schemas and the AWS resources are applied from the host, by `tools/scripts/db.sh` and `tools/scripts/aws.sh`, with the pgschema and the Terraform that `mise.toml` pins, so that each is pinned in one place (TPL-0010, TPL-0022).

These tests keep the behaviors of the emulator verified. They run in `make check-all`, and so in CI.

- The integration tests of `platform/awsevents` (`awsevents_integration_test.go`, in `make test-integration`) run the publisher and the consumer against the emulator. A rule on a custom bus delivers the `detail` of an event to an SQS queue whose policy allows `events.amazonaws.com`. An event whose handler keeps failing, and a message that is no event, move to the dead-letter queue after `maxReceiveCount` receives. `PutEvents` answers 200 with a failed entry for a source that AWS reserves, and the publisher reports it as an error. `Publisher.Ready` tells whether a bus exists.
- `make smoke` applies `infra/local/` with Terraform, starts the system, books, reschedules, and cancels a reservation, and waits for the three mails in Mailpit.
- `make e2e` starts the system in the same way, and a person in a browser registers a room, books it, reschedules, and cancels; the test waits for the three mails in Mailpit.

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

Facts verified with version 1.0.498, on November 18, 2026:

- The packages are RPM and deb files at `downloads.aws.com`, signed with a key of AWS. The comment at the top of the Dockerfile names the list of versions. There is no container image.
- The endpoint is HTTPS on port 9297, with a certificate for `localhost` and `127.0.0.1` that the emulator makes at every start, signed by a CA that it writes to its state directory.
- The emulator makes new credentials at every start. `--auth off` turns off the check of signatures.
- A rule on a custom bus delivers to an SQS queue only when the policy of the queue allows `events.amazonaws.com`, as in AWS. The input path `$.detail` works, and SQS moves a message to the dead-letter queue after `maxReceiveCount` receives.
- `PutEvents` answers 200 with a failed entry for a source that AWS reserves (one that starts with `aws.`).
- An event for a bus that does not exist is accepted and dropped. AWS refuses it.
- Terraform, with the AWS provider 6.73.0, applies `infra/local/` to the emulator. The provider waits about 25 seconds after it creates each SQS queue.
- Secrets Manager (creating and reading a secret) and `SendEmail` of SES v2 worked in a check by hand.
