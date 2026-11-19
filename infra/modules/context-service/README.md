# context-service

Everything of one bounded context on ECS (TPL-0014): the service of its API, if it has one, a service for each worker, the roles of its tasks, its logs, its security group, and its subscription to events (TPL-0013).
The processes are those of `compose.yaml`, from the same image and with the same commands.

| Process | ECS service | Command | Reached at |
|---|---|---|---|
| The API (`api`) | `<name>` | The image as it is | `http://<name>:8080`, through ECS Service Connect |
| A worker (`workers`) | `<name>-<key>` | `command`, such as `["relay"]`; an empty list runs the image as it is | Nowhere |

For the three contexts of the template:

| Context | `api` | `workers` | `publishes_events` | `subscription` |
|---|---|---|---|---|
| Facility | `{}` | | | |
| Reservation | `{}` | `{ relay = { command = ["relay"] } }` | `true` | |
| Notification | | `{ worker = { command = [] } }` | | `/contexts/reservation`, the confirmed and the cancelled reservations |

## Private by construction

Nothing that the module creates can be reached from outside the VPC, and no variable could change that: a task gets no public address, no service is behind a load balancer, and the security groups name no address range.
The tests in `tests/` check it, and review checks that only the module of the web front end creates a public entry.

- Every task runs in the private subnets, and joins two security groups: the group of the services, from the module [platform](../platform/README.md), and the group of the context, `<environment>-<name>`. The group of the context reaches PostgreSQL (5432) in the security group of the database, and the module adds the rule to the database group that accepts it.
- Every service enables ECS Service Connect, so every process can call the API of another context by name, such as `http://facility:8080`, the address that it has in Docker Compose too. The service of the API also serves under the name of the context, on its port `http` (8080).
- A context that needs more than this, such as a call to a service outside AWS, adds a rule to its own group (`security_group_id`).

## Configuration and secrets

A process reads its configuration from environment variables (TPL-0014), and every process of the context gets the same ones:

- `environment_variables`, such as `FACILITY_API_URL = "http://facility:8080"`.
- `AWS_REGION`, the Region of the provider.
- `DATABASE_URL`, from the key `url` of the secret `database.secret_arn`. ECS reads it from Secrets Manager when a task starts, so the code calls no AWS API to read it, and the task definition holds the ARN of the secret, not its value. The module rejects `DATABASE_URL` in `environment_variables`.
- `EVENT_BUS_NAME`, when `publishes_events` is true.
- `EVENTS_QUEUE_URL`, with a `subscription`: the queue that the module creates with the module [event-subscription](../event-subscription/README.md).

## Roles

- The execution role, with which ECS starts a task: it may pull from the repository `repository_arn` alone, write to the log group of the context, and read the secret of the database, with `kms:Decrypt` on `database.kms_key_arn` when a customer managed key encrypts the secret. The processes never get its permissions.
- The task role, with which the processes call AWS. It has only what the context calls: `events:DescribeEventBus` and `events:PutEvents` on the bus when `publishes_events` is true, `sqs:ReceiveMessage` and `sqs:DeleteMessage` on the queue of the context with a subscription, and the statements of `iam_statements`, such as `ses:SendEmail` for a context that sends mail through Amazon SES. Only ECS of the same account and Region may take it on.

The two roles belong to the context, so the API and the workers of one context share them.

## The tasks

- Fargate, in the network mode `awsvpc`, with `cpu_architecture` (`X86_64` by default).
- The health check of the image, `/service healthcheck`, for the API and the workers alike, with the timing of `backend/Dockerfile`.
- A read-only root file system and no Linux capability, as in `compose.yaml`; the image runs as a user other than root.
- The logs of every process and of its proxy go to the log group `/<environment>/<name>`, in streams that start with the name of the process, such as `api/` or `relay/`.
- A deployment whose tasks do not become healthy stops and rolls back (the deployment circuit breaker). There is no ECS Exec.
- The proxy of Service Connect runs in each task, beside the process, and shares its CPU and memory. AWS recommends 256 more CPU units and 64 MiB more memory for it; the defaults, 256 CPU units and 512 MiB, leave it a share of a small task.

Not yet applied to AWS. AWS Emulator accepts the module for the three contexts, wired with the modules network, registry, and platform, but it starts no task. That a task pulls its image, reads its secret, and reaches another context by name is checked when the module is first applied to AWS.

The tests in `tests/` plan the module against a mock of the AWS provider, so `make check-all` runs them without an AWS account (`validate-terraform`).

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.17 |
| aws | >= 6.0 |

## Providers

| Name | Version |
| ---- | ------- |
| aws | >= 6.0 |

## Modules

| Name | Source | Version |
| ---- | ------ | ------- |
| subscription | ../event-subscription | n/a |

## Resources

| Name | Type |
| ---- | ---- |
| [aws_cloudwatch_log_group.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_ecs_service.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_service) | resource |
| [aws_ecs_task_definition.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_task_definition) | resource |
| [aws_iam_role.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_iam_role_policy.task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_security_group.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_vpc_security_group_egress_rule.database](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_vpc_security_group_ingress_rule.database](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_ingress_rule) | resource |
| [aws_caller_identity.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [aws_iam_policy_document.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.execution_assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.task_assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_partition.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/partition) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| api | The API of the context, or null for a context without one, such as Notification. The API runs as the ECS service `<name>`, from the image with no command, and the other services call it at `http://<name>:8080`. `cpu` and `memory` are those of one task, in CPU units and MiB, as Fargate allows them; the proxy of ECS Service Connect runs in the task too. | ```object({ desired_count = optional(number, 1) cpu = optional(number, 256) memory = optional(number, 512) })``` | `null` | no |
| cluster\_arn | The ARN of the ECS cluster: `cluster_arn` of the module platform. | `string` | n/a | yes |
| cpu\_architecture | The CPU architecture of the image: `X86_64` or `ARM64`. Fargate on ARM64 (Graviton) costs less for the same size of task. | `string` | `"X86_64"` | no |
| database | The database of the context. `security_group_id` is the security group of the database: the module allows PostgreSQL (5432) from the tasks of the context into it. `secret_arn` is the ARN of a secret of Secrets Manager whose value is JSON with the key `url`, the URL of the runtime role of the context, which the processes get as DATABASE\_URL. `kms_key_arn` is the ARN of the customer managed key of AWS KMS that encrypts the secret, if one does; null for the key that AWS manages for Secrets Manager. | ```object({ security_group_id = string secret_arn = string kms_key_arn = optional(string) })``` | n/a | yes |
| environment | The name of the environment, such as `stg`. The task definitions, the roles, the log group, and the security group of the context are named after it. | `string` | n/a | yes |
| environment\_variables | Environment variables of every process of the context, such as `{ FACILITY_API_URL = "http://facility:8080" }`. The module sets AWS\_REGION, DATABASE\_URL (from the secret), EVENT\_BUS\_NAME (when the context publishes events), and EVENTS\_QUEUE\_URL (with a subscription) itself. A secret belongs in Secrets Manager, not here: the values are plain in the task definition. | `map(string)` | `{}` | no |
| event\_bus | The event bus of the environment: `event_bus` of the module platform. | ```object({ name = string arn = string })``` | n/a | yes |
| iam\_statements | More permissions of the role of the tasks, beyond the events and the queue, such as `[{ actions = ["ses:SendEmail"], resources = ["<ARN of the identity>"] }]` for a context that sends mail through Amazon SES. | ```list(object({ actions = list(string) resources = list(string) }))``` | `[]` | no |
| image | The image that every process of the context runs, with a tag or a digest, such as `<account>.dkr.ecr.<region>.amazonaws.com/reservation:<tag>`: built from `backend/Dockerfile` with the build argument CONTEXT, and pushed to a repository of the module registry. | `string` | n/a | yes |
| log\_retention\_days | How many days CloudWatch Logs keeps the logs of the context. 0 keeps them for ever. | `number` | `30` | no |
| name | The name of the context, such as `reservation`: the name of its command in `backend/cmd/`. The service of its API has this name, and the other contexts call the API at `http://<name>:8080`. | `string` | n/a | yes |
| namespace\_arn | The ARN of the namespace of ECS Service Connect: `namespace_arn` of the module platform. | `string` | n/a | yes |
| publishes\_events | Whether the context publishes integration events (TPL-0013). When true, the processes get EVENT\_BUS\_NAME, and the role of the tasks may put events on the bus and describe it: the relay checks at its start that the bus exists. | `bool` | `false` | no |
| repository\_arn | The ARN of the Amazon ECR repository of the image: `repository_arns` of the module registry. The tasks may pull from this repository alone. | `string` | n/a | yes |
| services\_security\_group\_id | The ID of the security group that every task of every service joins: `services_security_group_id` of the module platform. | `string` | n/a | yes |
| subnet\_ids | The IDs of the private subnets in which the tasks run: `private_subnet_ids` of the module network. | `list(string)` | n/a | yes |
| subscription | The events that the context consumes, or null for a context that consumes none (TPL-0013). `sources` are the CloudEvents `source` of each publishing context, such as `/contexts/reservation`, and `event_types` the CloudEvents `type` of each event, such as `reservation.ReservationConfirmed.v1`. The module creates the queue of the context with the module event-subscription, gives the processes its URL as EVENTS\_QUEUE\_URL, and lets the role of the tasks receive and delete its messages. | ```object({ sources = list(string) event_types = list(string) })``` | `null` | no |
| vpc\_id | The ID of the VPC: `vpc_id` of the module network. | `string` | n/a | yes |
| workers | The workers of the context, by name, such as `relay`: processes that serve no requests (TPL-0014). Each runs as the ECS service `<name>-<key>`, from the image of the context. `command` is the arguments of the image, such as `["relay"]`; an empty list runs the image as it is, as the worker of Notification does. `cpu` and `memory` are those of one task, in CPU units and MiB, as Fargate allows them; the proxy of ECS Service Connect runs in the task too. | ```map(object({ command = list(string) desired_count = optional(number, 1) cpu = optional(number, 256) memory = optional(number, 512) }))``` | `{}` | no |

## Outputs

| Name | Description |
| ---- | ----------- |
| api\_url | The URL at which the other services call the API of the context, such as `http://reservation:8080`, through ECS Service Connect; null when the context has no API. |
| events\_queue\_url | The URL of the queue of the events that the context consumes, which its processes get as EVENTS\_QUEUE\_URL; null without a subscription. |
| log\_group\_name | The name of the CloudWatch log group of every process of the context, such as `/stg/reservation`. |
| security\_group\_id | The ID of the security group of the context, which its tasks join besides the group of the services. A rule that only this context needs, such as one to a service outside AWS, belongs here. |
| service\_names | The names of the ECS services of the context: `<name>` for the API, and `<name>-<key>` for each worker. |
| task\_role\_arn | The ARN of the role with which the processes of the context call AWS. |
<!-- END_TF_DOCS -->
