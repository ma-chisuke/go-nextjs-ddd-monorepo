---
status: accepted
date: 2026-11-19
---

# TPL-0014: Deploy each context to ECS on its own, with private APIs and one public entry

## Context and Problem Statement

The backend is one Go module, split by bounded context (TPL-0003).
The contexts can still be deployed together, as one program, or apart, as one program per context.

Deploying them apart serves the purpose of the template.
A context that is built, released, scaled, and rolled back on its own is a context that one team can own, and a fault or a deploy in one context does not stop the others.
It also shows that the boundaries hold: contexts that talk only through APIs and events (TPL-0005) can run as separate processes.

Requests also need a way in.
The browser talks to Next.js, and Next.js and other contexts call the APIs of the contexts.
Those APIs must not be reachable from the internet; only Next.js should be.

On AWS, the usual way to give each service an entry is a load balancer or a listener rule, a target group, security groups, and DNS for each service, which is a lot of infrastructure to repeat for every context.
On October 20, 2026, AWS released Amazon ECS Gateway, a managed ingress for ECS services.

Where do the contexts and Next.js run, and how do requests reach them?

## Decision Drivers

- A context is built, deployed, scaled, and rolled back without the others.
- The APIs of the contexts are private. Only the web front end is public.
- Little infrastructure is repeated for each context, and there is no cluster to operate.
- The processes are long-running: an API server, and a worker that reads SQS (TPL-0013), both with a pool of database connections (TPL-0016).
- Terraform creates every part and wires every address, so that an environment comes from the code alone, and the checks can test that code without an AWS account (TPL-0015).

## Considered Options

- An ECS service on Fargate for each context, with private APIs that the services call through ECS Service Connect, and one public load balancer for the web front end
- An ECS service on Fargate for each context, behind ECS Gateway
- One deployable for all contexts (a modular monolith)
- Lambda with API Gateway
- Kubernetes (Amazon EKS)
- An ECS service for each context, with a load balancer for each service

## Decision Outcome

Chosen option: "An ECS service on Fargate for each context, with private APIs that the services call through ECS Service Connect, and one public load balancer for the web front end", because it deploys each context on its own without servers or a cluster to run, gives each API the address in AWS that it has in Docker Compose, and is made of resources that the AWS provider of Terraform has today.

The rules are as follows.

- Each context runs its API, if it has one, as the ECS service `<context>` on Fargate. A worker, a process that serves no requests, such as the relay of an outbox or the consumer of a queue (TPL-0013), runs as an ECS service of its own, `<context>-<worker>`, from the image of its context; the worker of Notification is the service `notification-worker`. The services of one context scale and deploy separately, and a worker publishes no port, so it has no entry for requests.
- A worker is a mode of the command of its context, which the same binary runs when the name of the mode is its argument, such as `reservation relay`. A context without an API, such as Notification, runs its worker as the command itself (`notification`).
- Every process serves `GET /healthz`, a worker too (`httpserver.RunWorker`), so the health check command of the image, `<command> healthcheck`, watches every process the same way. The health check of a worker says only that the process runs. A worker that cannot reach its database or its broker stays healthy and tries again, because a restart would not bring them back.
- A service reads its configuration from environment variables, its secrets included, such as the password in `DATABASE_URL` (`platform/config`). In AWS, the task definition fills the secrets from Secrets Manager, so the code calls no AWS API to read them.
- Each context has its own container image, built from `backend/cmd/<context>/`. The Go module stays one.
- The tag of an image is a hash of the files that it is built from (`tools/scripts/images.sh`). The files of a context are complete because a context imports nothing of another context (`make lint-arch`): its command, its own packages, the platform packages, and the module files. A change to one context therefore changes the task definitions of that context alone, and a deployment restarts only the contexts that changed.
- The deploy job builds the images for `linux/amd64`, once for each environment, from the same commit. Promoting one image by its digest from stg to prd is the alternative, and it is not built.
- Each context has a Terragrunt unit, `contexts/<context>`, with one call of the module `context-service` (TPL-0015). The services of a context share one task execution role, with which ECS starts the tasks, and one task role, with which the processes call AWS. The task role has only what the context calls, such as `ses:SendEmail` on the identity of the environment for Notification.
- A deployment whose tasks do not become healthy stops, and ECS rolls the service back to the last deployment that worked (the deployment circuit breaker).
- Next.js runs on ECS on Fargate as well, from an image of its own, `frontend/Dockerfile`.
- Only the server of Next.js calls the APIs of the contexts. The browser talks to Next.js alone, so no API needs CORS, and no address of an API reaches the browser. The clients of the APIs live in `frontend/src/lib/api/server/clients.ts`, which imports `server-only`, and the server reads the base URL of each API from the environment (`FACILITY_API_URL`, `RESERVATION_API_URL`), so one image runs in every environment.
- The health check of the frontend, `GET /healthz`, calls no API, as that of a context calls no other service: a context that is down must not make the platform restart the frontend. It answers 200 with a body, not 204, because Next.js renders the answer at build time. The image has no shell, so Node runs the check (`frontend/healthcheck.mjs`), in Docker Compose and in ECS.
- The APIs of the contexts are private. A context, and the server of Next.js, call an API through ECS Service Connect at `http://<context>:8080`, the address that it has in Docker Compose. Every service is a client of the namespace of its environment, and the service of an API also serves under the name of its context. No API has a load balancer or a public address, and the module `context-service` has no input that could give it either.
- Only the web front end is public. One load balancer in the public subnets, from the module `web`, serves HTTPS and forwards to the tasks of Next.js, which accept requests from that load balancer alone.
- Services use the network mode `awsvpc`, which Fargate requires.
- Notification sends its mail through Amazon SES, with its task role. It speaks SMTP only in the local environment, to Mailpit (TPL-0017).

The tasks reach AWS through VPC endpoints (AWS PrivateLink), and not through the internet.

- By default, the private subnets of the tasks have no route to the internet. A gateway endpoint serves Amazon S3, and an interface endpoint serves each of `ecr.api`, `ecr.dkr`, `logs`, `secretsmanager`, `ecs-agent` (which the proxy of Service Connect uses), `events`, `sqs`, and `email` (the API of Amazon SES).
- So no rule of a security group names an address range outside the VPC: a task may send HTTPS to the security group of the endpoints and to the prefix list of Amazon S3, and nowhere else outside the VPC.
- The documentation of Amazon ECS calls a NAT gateway the easiest way to reach the AWS services, and names its cost: a NAT gateway cannot limit the destinations that a task talks to. The security pillar of the AWS Well-Architected Framework recommends VPC endpoints as the default way to reach managed services, and Trivy rates egress to `0.0.0.0/0` as critical (AWS-0104, TPL-0024). The template follows the recommendation of AWS.
- A NAT gateway remains an option of the module `network` (`nat_gateways`), for a context that calls a service outside AWS.

Two parts of this decision differ from the first plan.

- The first plan gave every context an API, with a worker next to it. Notification has no API, so its command is its worker.
- The first plan put the APIs behind a private ECS Gateway and Next.js behind a public one. The infrastructure milestone set ECS Gateway aside, because Terraform could neither create a gateway with the AWS provider nor read the address that a gateway gives a service, and nothing here could test it ([ECS Gateway](#an-ecs-service-on-fargate-for-each-context-behind-ecs-gateway) below).

### Consequences

- Good, because each context deploys, scales, and fails on its own, and a deployment restarts only the contexts whose image changed.
- Good, because the boundaries hold at run time too: the contexts are separate processes, so one reaches another only through its API or its events.
- Good, because an API has the same address in AWS and in Docker Compose, such as `http://facility:8080`, so a setting that names an API, such as `FACILITY_API_URL`, is the same in both.
- Good, because the AWS provider of Terraform has every resource of the decision, so Terraform wires every address, and its tests, the validation of every unit, and the rehearsal on AWS Emulator check the code without an account (TPL-0015, TPL-0017).
- Good, because no API can be reached from outside the VPC, and the security groups of the tasks name no address range outside it.
- Bad, because there are more services than with one deployable: more images, task definitions, logs, and alarms, and a minimum cost for each service, since a Fargate task is charged while it runs, even when idle.
- Bad, because the traffic between tasks inside the VPC is plain HTTP. Service Connect can encrypt it with TLS from AWS Private CA, which is not set up.
- Bad, because the load balancer of the one public entry, with its listeners, its target group, its certificate, and its DNS record, is written by hand in the module `web`, where ECS Gateway would have provisioned it.
- Bad, because the proxy of Service Connect runs in every task and shares the CPU and the memory of the task.
- Bad, because the interface endpoints are charged by the hour for each zone, and for a small environment they cost more than one NAT gateway. stg keeps one network interface for each endpoint (`interface_endpoints_in_every_zone = false`), which costs less and fails with its zone; prd keeps one in every zone.
- Bad, because a task reaches only the AWS services that have an endpoint. A new service that the tasks call needs one more entry in `interface_endpoints`, and a service outside AWS needs a NAT gateway and a rule in the security group of the context.
- Bad, because a call between contexts crosses the network, with latency and failures that a call inside one process does not have.
- Bad, because nothing has run on AWS yet. That a task pulls its image through the endpoints, reads its secret, and reaches another context by name is checked when an environment is first deployed ([Deploy to AWS](../../guides/deploy.md#what-the-first-apply-must-confirm)).

### Confirmation

- `backend/Dockerfile` builds the image of the context that the build argument `CONTEXT` names, from `backend/cmd/<context>/`, so a new context needs no Dockerfile of its own and the Go module stays one.
- `make smoke`, part of `make check-all`, runs every process of every context as a separate container in Docker Compose: the APIs of Facility and Reservation, the relay of Reservation (`reservation relay`, from the image of Reservation), and the worker of Notification. It books a room across the APIs over HTTP, and waits for the mails that the events of the booking cause. The contexts run as separate processes and reach each other only through their APIs and their events.
- The binary in each image checks its own health (`<command> healthcheck`), for an API and for a worker alike, and the task definitions of `context-service` run that command. The tests of `httpserver.RunWorker` check that a worker answers the health check while it runs.
- The frontend has an image of its own too, and Docker Compose runs it as the service `web`, which reaches the APIs through the network of Compose, as it reaches them through Service Connect in AWS. `make e2e` drives it in a browser, and one of its tests checks that the health check answers.
- `lint-frontend` fails on a module under `frontend/src/**/server/` that does not import `server-only`, and on a client component that imports such a module (`no-restricted-syntax`). The build of Next.js fails when a client component imports `server-only`.
- `lint-arch`, in `make check`, fails on an import of another context, which keeps the files behind the tag of an image complete.
- The tests of `infra/modules/context-service` (`validate-terraform`, in `make check-all`) plan the module against a mock of the AWS provider. They assert that a service runs in the private subnets with no public address and no load balancer, that a rule of the context names a security group and never an address range, that the API serves through Service Connect under the name of its context, and that a deployment that fails rolls back.
- `scan-config`, in `make check`, fails on a load balancer that faces the internet in any file but `infra/modules/web/main.tf`, the one file for which `.trivyignore.yaml` accepts the finding AWS-0053, and on a rule that allows egress to `0.0.0.0/0` (AWS-0104).
- `validate-live` and `rehearse-infra`, in `make check-all`, validate every unit and apply the units of stg to AWS Emulator (TPL-0015, TPL-0017).
- Review checks that only `infra/modules/web` creates a public entry.

## Pros and Cons of the Options

### An ECS service on Fargate for each context, with private APIs that the services call through ECS Service Connect, and one public load balancer for the web front end

- Good, because contexts deploy on their own, there is no cluster to operate, the APIs are private by where they run and by what the module accepts, and an API has the address that it has in Docker Compose.
- Good, because every resource is in the AWS provider of Terraform, and AWS Emulator accepts every one.
- Bad, because each service has a minimum cost, the traffic inside the VPC is plain HTTP, and the one public entry is written by hand.

### An ECS service on Fargate for each context, behind ECS Gateway

This option was chosen first. These facts were checked on November 19, 2026.

- Good, because a gateway is a managed ingress: it is defined once, services opt in, and services share load balancers, with host-based routing and generated hostnames. There is no charge beyond the load balancers that it provisions.
- Bad, because the AWS provider of Terraform has no resource for ECS Gateway. Version 6.73.0, released on November 11, 2026, is the latest, and the request for the resources is open ([issue 50573](https://github.com/hashicorp/terraform-provider-aws/issues/50573), opened on October 21, 2026), with three open pull requests (50894, 50895, and 50896) and no milestone.
- Bad, because Terraform could not wire the addresses. The provider `awscc`, from version 1.107.0, has `awscc_ecs_gateway`, `awscc_ecs_gateway_cluster_associations`, and `gateway_connectors` on `awscc_ecs_service`, but neither it nor CloudFormation exposes the URL that a gateway gives a service, `https://<random id>.ecs.<region>.on.aws`. Only the ECS API returns it (`describe-service-revisions`, `describe-gateway-route`), and a service that is deleted and created again gets another URL. So Terraform could give neither Reservation the address of Facility nor Next.js the addresses of both APIs.
- Bad, because support for custom domain names is not documented, so the public entry of Next.js could not be shown to get the team's own domain name.
- Bad, because AWS Emulator answers `CreateGateway` with "scaffolded but not implemented", so no check of this repository could apply a gateway.
- Bad, because the product is new: it was released on October 20, 2026, and its limits in production are not yet known.

### One deployable for all contexts (a modular monolith)

- Good, because one image, one service, and one deploy are the cheapest and simplest to run, and calls between contexts can stay inside the process.
- Bad, because every deploy ships every context, so a fault in the release of one context rolls back all of them, and the contexts cannot scale apart.
- Bad, because nothing at run time shows that the contexts are independent. A shortcut through shared memory or a shared connection pool is easy to add and hard to see.

### Lambda with API Gateway

- Good, because it costs nothing while idle, there is no server to manage, and it scales with requests.
- Bad, because each instance of a function opens its own database connections, so a burst of instances can use up the connections of PostgreSQL. The usual remedy, RDS Proxy, keeps an Aurora Serverless v2 cluster from pausing (TPL-0016).
- Bad, because the code and the local environment are shaped around the Lambda runtime and its events, while the same Go server runs unchanged on ECS and in Docker Compose.

### Kubernetes (Amazon EKS)

- Good, because it is the most portable and flexible platform, with the largest ecosystem.
- Bad, because each EKS cluster has an hourly charge for its control plane, on top of the capacity it runs.
- Bad, because Kubernetes needs skills and upkeep (upgrades, add-ons, manifests) far beyond what a template for small teams should ask for.

### An ECS service for each context, with a load balancer for each service

- Good, because it is the long-proven way to expose ECS services, with plenty of documentation and examples.
- Bad, because every service repeats a load balancer or a listener rule, a target group, security groups, and DNS, and each load balancer is charged by the hour.
- Bad, because the APIs need no entry from outside the VPC, so a load balancer in front of each would be infrastructure without a use. Service Connect gives them names inside the VPC instead.

## More Information

Facts about Amazon ECS Gateway, as of November 2026, from [its documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/gateway-connect-service.html):

- It was launched on October 20, 2026.
- It is a managed ingress: a gateway is defined once, and services opt in.
- There is no extra charge beyond the load balancers that it provisions.
- Services share load balancers, with host-based routing and generated hostnames.
- It requires the network mode `awsvpc`.
- Services in ECS Express Mode cannot connect to it.
- Support for custom domain names is not documented.

Revisit this decision when the AWS provider of Terraform has the resources of ECS Gateway, and Terraform can read the URL that a gateway gives a service.

Not yet verified: the service name of the endpoint of the API of Amazon SES, `email`. The documentation of Amazon SES says to filter the endpoint services for "email", and does not spell the name out; AWS Emulator does not know it, so the rehearsal leaves it out. It is checked when an environment is first deployed.

- The modules: `infra/modules/network`, `platform`, `context-service`, and `web`, each with a README.
- [Deploy to AWS](../../guides/deploy.md) prepares an account and describes a deployment.
