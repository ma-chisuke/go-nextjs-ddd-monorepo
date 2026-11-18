---
status: accepted
date: 2026-11-17
---

# TPL-0014: Deploy each context to ECS on its own, behind ECS Gateway

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

## Considered Options

- An ECS service on Fargate for each context, behind ECS Gateway
- One deployable for all contexts (a modular monolith)
- Lambda with API Gateway
- Kubernetes (Amazon EKS)
- An ECS service for each context, with a load balancer for each service

## Decision Outcome

Chosen option: "An ECS service on Fargate for each context, behind ECS Gateway", because it deploys each context on its own without servers or a cluster to run, and ECS Gateway gives every service an entry without a load balancer for each service.

The rules are as follows.

- Each context runs its API, if it has one, as an ECS service on Fargate. A worker, a process that serves no requests, such as the relay of an outbox or the consumer of a queue (TPL-0013), runs as an ECS service of its own, from the image of its context. The services of one context scale and deploy separately, and a worker attaches to no gateway, so it has no entry for requests.
- A worker is a mode of the command of its context, which the same binary runs when the name of the mode is its argument, such as `reservation relay`. A context without an API, such as Notification, runs its worker as the command itself (`notification`).
- Every process serves `GET /healthz`, a worker too (`httpserver.RunWorker`), so the health check command of the image, `<command> healthcheck`, watches every process the same way. The health check of a worker says only that the process runs. A worker that cannot reach its database or its broker stays healthy and tries again, because a restart would not bring them back.
- A service reads its configuration from environment variables, its secrets included, such as the password in `DATABASE_URL` (`platform/config`). In AWS, the task definition fills the secrets from Secrets Manager, so the code calls no AWS API to read them.
- Each context has its own container image, built from `backend/cmd/<context>/`. The Go module stays one.
- Next.js runs on ECS on Fargate as well, from an image of its own, `frontend/Dockerfile`.
- Only the server of Next.js calls the APIs of the contexts. The browser talks to Next.js alone, so no API needs CORS, and no address of an API reaches the browser. The clients of the APIs live in `frontend/src/lib/api/server/clients.ts`, which imports `server-only`, and the server reads the base URL of each API from the environment (`FACILITY_API_URL`, `RESERVATION_API_URL`), so one image runs in every environment.
- The health check of the frontend, `GET /healthz`, calls no API, as that of a context calls no other service: a context that is down must not make the platform restart the frontend. It answers 200 with a body, not 204, because Next.js renders the answer at build time. The image has no shell, so Node runs the check (`frontend/healthcheck.mjs`), in Docker Compose and in ECS.
- Requests enter through Amazon ECS Gateway. The APIs of the contexts attach to a private gateway, and the Terraform code writes the access type `PRIVATE` explicitly instead of relying on a default. Only Next.js is public.
- Services use the network mode `awsvpc`, which ECS Gateway requires. They do not use ECS Express Mode, because services in Express Mode cannot connect to ECS Gateway.

This differs from the first plan in one point: the first plan gave every context an API, with a worker next to it. Notification has no API, so its command is its worker.

### Consequences

- Good, because each context deploys, scales, and fails on its own, and a deploy of one context does not restart the others.
- Good, because the boundaries hold at run time too: the contexts are separate processes, so one reaches another only through its API or its events.
- Good, because ECS Gateway removes the work of a load balancer for each service. A gateway is defined once, services opt in, and services share load balancers with host-based routing and generated hostnames. There is no charge for ECS Gateway beyond the load balancers that it provisions.
- Bad, because there are more services than with one deployable: more images, task definitions, logs, and alarms, and a minimum cost for each service, since a Fargate task is charged while it runs, even when idle.
- Bad, because ECS Gateway is new. It was released on October 20, 2026, and its limits in production are not yet known. One gap is already visible: see More Information on custom domain names.
- Bad, because a call between contexts crosses the network, with latency and failures that a call inside one process does not have.

### Confirmation

Since M1, each context has an image of its own, and nothing yet runs it on ECS.

- `backend/Dockerfile` builds the image of the context that the build argument `CONTEXT` names, from `backend/cmd/<context>/`, so a new context needs no Dockerfile of its own and the Go module stays one.
- `make smoke`, part of `make check-all`, runs every process of every context as a separate container in Docker Compose: the APIs of Facility and Reservation, the relay of Reservation (`reservation relay`, from the image of Reservation), and the worker of Notification. It books a room across the APIs over HTTP, and waits for the mails that the events of the booking cause. The contexts run as separate processes and reach each other only through their APIs and their events.
- The binary in each image checks its own health (`<command> healthcheck`), for an API and for a worker alike, which is the command that the health check of an ECS task will run. The tests of `httpserver.RunWorker` check that a worker answers the health check while it runs.
- Since the frontend milestone, the frontend has an image of its own too, and Docker Compose runs it as the service `web`, which reaches the APIs through the network of Compose, as it will reach them through the private network. `make e2e` drives it in a browser, and one of its tests checks that the health check answers.
- `lint-frontend` fails on a module under `frontend/src/**/server/` that does not import `server-only`, and on a client component that imports such a module (`no-restricted-syntax`). The build of Next.js fails when a client component imports `server-only`.

The following checks arrive with the infrastructure milestone.

- The Terraform module for a context service (`infra/modules/context-service`) accepts only the private access type, so a context API cannot be made public by changing a value.
- tflint and Trivy check the Terraform code.
- Review checks that only the module for Next.js (`infra/modules/web`) creates a public entry.

Until then, review enforces the rest of the decision.

## Pros and Cons of the Options

### An ECS service on Fargate for each context, behind ECS Gateway

- Good, because contexts deploy on their own, there is no cluster to operate, and one gateway definition serves every service.
- Bad, because each service has a minimum cost, and ECS Gateway is new.

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
- Bad, because sharing one load balancer between services is possible, but then the team manages the listener rules by hand, which is what ECS Gateway does for it.

## More Information

Facts about Amazon ECS Gateway, as of November 2026, from [its documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/gateway-connect-service.html):

- It was launched on October 20, 2026.
- It is a managed ingress: a gateway is defined once, and services opt in.
- There is no extra charge beyond the load balancers that it provisions.
- Services share load balancers, with host-based routing and generated hostnames.
- It requires the network mode `awsvpc`.
- Services in ECS Express Mode cannot connect to it.
- Support for custom domain names was not documented at launch.

Not yet verified: how the public entry of Next.js gets the team's own domain name through ECS Gateway. This is checked when the infrastructure is built.
