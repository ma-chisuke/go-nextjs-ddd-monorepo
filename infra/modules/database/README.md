# database

The database of one environment: an Aurora PostgreSQL cluster with one database that every bounded context shares, the secrets of the two roles of each context, and the ECS task that prepares the database at deploy time (TPL-0009, TPL-0016).

| Part | What it does |
|---|---|
| Cluster | Aurora PostgreSQL in the database subnets, encrypted with a key of the account, protected from deletion, with automated backups |
| Security group of the cluster | Accepts PostgreSQL only from the security groups that a rule names; each context adds the rule for its own tasks |
| Secrets of the roles | `<environment>/database/<context>/owner` and `<environment>/database/<context>/runtime` for each context, as JSON with the keys `username`, `password`, and `url` |
| Task that prepares the database | Runs the image of `db/Dockerfile` once at each deploy, before the services start |

## Capacity

`capacity` selects exactly one kind of capacity.

- `serverless = { min_acu, max_acu, seconds_until_auto_pause }`: Aurora Serverless v2. The default is 0 to 2 ACU: with a minimum of 0, the cluster pauses after `seconds_until_auto_pause` seconds without a connection (300 by default), and an idle environment pays for storage only. This suits stg.
- `provisioned = { instance_class }`: instances of a fixed class, such as `db.r8g.large`, which cost less than Serverless v2 under a steady load. prd chooses by its load.

The validation of the variable rejects both kinds or neither, a minimum above the maximum, a capacity off the steps of 0.5 ACU, and a time to pause outside 300 to 86400 seconds.
`instance_count` adds readers; one of them takes over when the writer fails.

A cluster pauses only while no connection is open, so what connects to it follows the rules of TPL-0016.

- The connection pool of a service keeps no idle connection (`backend/internal/platform/postgres/pool.go`).
- The health check of a service does not touch the database.
- No RDS Proxy, which keeps a connection open to each instance, and no logical replication.
- Resuming takes about 15 seconds, and 30 or more after a pause longer than a day. A connection waits that long and tries again: the pool of a service waits 30 seconds, and the task that prepares the database tries five times.

## How a context uses the database

The unit of a context gives its service two things from the outputs of this module.

- The environment variable `DATABASE_URL`, from the key `url` of the secret of its runtime role: `valueFrom = "<runtime_secret_arns[context]>:url::"` in the task definition, and `secretsmanager:GetSecretValue` on that ARN in the execution role. The URL connects with TLS (`sslmode=require`).
- An ingress rule in the security group of the cluster (`security_group_id`), from the security group of its tasks, on port 5432.

The secrets use the key that Secrets Manager manages for the account, so a role that may read a secret needs no permission on a key.

## Passwords

The roles sign in with passwords, which ECS reads from Secrets Manager into the environment of a task (TPL-0014); no IAM database authentication.

- The password of each role never enters the state of Terraform. The ephemeral resource `random_password` makes it at each run, and the write-only argument `secret_string_wo` sends it to Secrets Manager only when `password_version` changes. A password has letters and digits only, so it stands in the URL as it is.
- RDS makes the password of the master user, keeps it in a secret of its own, and rotates it (`manage_master_user_password`). Only the task that prepares the database signs in as the master user.
- To rotate the passwords of the roles, raise `password_version` and apply, run the task that prepares the database, then start the services again. Between the task and the new start, a service still holds the old password, and its new connections fail.

## The task that prepares the database

The image of `db/Dockerfile` runs `db/prepare.sh`: it waits for a cluster that may be resuming, runs `db/bootstrap.sql` as the master user, sets the password of each role from its secret, and applies the `schema.sql` of each context with pgschema as its owner role (TPL-0010). Each step is safe to run again.
The deploy runs it once, before the services start, with `aws ecs run-task` and the output `db_task`: launch type Fargate, the subnets and the security group of `db_task`, no public address. It then waits until the task stops, and fails unless the container exited with 0.

- The task reaches the cluster on port 5432, and the AWS services through the VPC endpoints of the network: HTTPS to the security group of the endpoints and to the prefix list of Amazon S3. No rule names an address range.
- The execution role pulls from the repository of `db_image` only, writes to the log group `/<environment>/db`, and reads exactly the secrets of the task. The role of the task has no permission.
- The root file system is read-only: the script writes no file.
- The master user of Aurora is not a superuser. It owns the database and may create roles, so `db/bootstrap.sql` makes it a member of each owner role that it creates; PostgreSQL 16 and later require that membership to hand a schema to the role. `make test-db-image` runs the image against the local PostgreSQL as such a role.

## Deleting the cluster

`deletion_protection` is on by default. To destroy the cluster on purpose, turn it off and apply first.
A destroy leaves a final snapshot, `<environment>-final`; a second destroy of a cluster of the same name fails until that snapshot is deleted.
Secrets Manager keeps a deleted secret for 30 days, during which a secret of the same name cannot be created.

## Not yet applied to AWS

The module has been applied to AWS Emulator only (TPL-0017), with the engine version 17.9, the newest that it knows. It accepted every resource; it gives the cluster a port of its own, so a plan after the apply shows a change of the port.
These facts about AWS are assumed, and checked when the module is first applied.

- Aurora PostgreSQL offers 18.6, and its Serverless v2 accepts a minimum of 0 ACU from 18.3 (TPL-0016 leaves this open).
- The master user of Aurora PostgreSQL 18 has CREATEROLE and CREATEDB, owns the database that the cluster creates, holds the ADMIN OPTION on a role that it creates, as PostgreSQL 16 and later give the creator, and may install the extension `btree_gist`.
- The secret of the master user holds the keys `username` and `password`, and a role reads it, and the secrets of this module, without a permission on a KMS key.
- ECS fills one variable from one key of a JSON secret with `"<ARN>:<key>::"`, for the secret that RDS manages too.

The tests in `tests/` plan the module against a mock of the AWS provider, and apply to that mock where an assertion needs a value that AWS computes, so `make check-all` runs them without an AWS account (`validate-terraform`).

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.17 |
| aws | >= 6.0 |
| random | >= 3.7 |

## Providers

| Name | Version |
| ---- | ------- |
| aws | >= 6.0 |

## Modules

No modules.

## Resources

| Name | Type |
| ---- | ---- |
| [aws_cloudwatch_log_group.task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_db_subnet_group.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/db_subnet_group) | resource |
| [aws_ecs_task_definition.db](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_task_definition) | resource |
| [aws_iam_role.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_kms_alias.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_alias) | resource |
| [aws_kms_key.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_key) | resource |
| [aws_rds_cluster.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/rds_cluster) | resource |
| [aws_rds_cluster_instance.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/rds_cluster_instance) | resource |
| [aws_secretsmanager_secret.role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/secretsmanager_secret) | resource |
| [aws_secretsmanager_secret_version.role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/secretsmanager_secret_version) | resource |
| [aws_security_group.cluster](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_security_group.task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_vpc_security_group_egress_rule.task_endpoints](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_vpc_security_group_egress_rule.task_postgres](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_vpc_security_group_egress_rule.task_s3](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_vpc_security_group_ingress_rule.cluster_from_task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_ingress_rule) | resource |
| [aws_iam_policy_document.ecs_tasks_assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_partition.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/partition) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| backup\_retention\_days | How many days Aurora keeps the automated backups, from which the cluster can be restored to any second. | `number` | `7` | no |
| capacity | The capacity of the instances, exactly one of two kinds (TPL-0016). `serverless = { min_acu, max_acu, seconds_until_auto_pause }` is Aurora Serverless v2, between `min_acu` and `max_acu` Aurora capacity units; with `min_acu = 0` the cluster pauses after `seconds_until_auto_pause` seconds without a connection (300 to 86400, by default 300), and resumes on the next connection. `provisioned = { instance_class }` is instances of a fixed class, such as `db.r8g.large`. The default is Serverless v2 from 0 to 2 ACU, which costs nothing but storage while it pauses. | ```object({ serverless = optional(object({ min_acu = number max_acu = number seconds_until_auto_pause = optional(number, 300) })) provisioned = optional(object({ instance_class = string })) })``` | ```{ "serverless": { "max_acu": 2, "min_acu": 0 } }``` | no |
| contexts | The bounded contexts, such as `["facility", "reservation", "notification"]`: the directories of `backend/internal/` with a `schema.sql`. Each one gets two secrets, for its roles `<context>_owner` and `<context>_runtime` (TPL-0009). | `set(string)` | n/a | yes |
| database\_name | The name of the database that every context shares (TPL-0009). | `string` | `"app"` | no |
| db\_image | The image of `db/Dockerfile` in Amazon ECR, best with its digest: `<account>.dkr.ecr.<region>.amazonaws.com/<repository>@sha256:<digest>`. The task that prepares the database runs it. | `string` | n/a | yes |
| deletion\_protection | Whether AWS refuses to delete the cluster. Turn it off, and apply, before you destroy the cluster on purpose. | `bool` | `true` | no |
| endpoints\_security\_group\_id | The ID of the security group of the interface endpoints (`endpoints_security_group_id` of the module network). The task reaches Amazon ECR, CloudWatch Logs, and Secrets Manager through them. | `string` | n/a | yes |
| engine\_version | The version of Aurora PostgreSQL, such as `18.6`. A version below 18 does not run the schemas of the template: they call `uuidv7()`, which PostgreSQL 18 brought (TPL-0009). | `string` | `"18.6"` | no |
| environment | The name of the environment, such as `stg`. Every resource is named after it. | `string` | n/a | yes |
| instance\_count | How many instances the cluster has: the writer, and readers after it. A reader takes over when the writer fails. | `number` | `1` | no |
| log\_retention\_days | How many days CloudWatch Logs keeps what the task that prepares the database printed. | `number` | `30` | no |
| password\_version | The version of the passwords of the roles of the contexts. Raising it writes a new password to each secret; the task that prepares the database then sets them on the roles, and the services read them when they start again. | `number` | `1` | no |
| s3\_prefix\_list\_id | The ID of the prefix list of Amazon S3 (`s3_prefix_list_id` of the module network). The task pulls the layers of its image from there. | `string` | n/a | yes |
| subnet\_ids | The IDs of the database subnets, where the cluster runs (`database_subnet_ids` of the module network). Aurora needs subnets in two Availability Zones at least. | `list(string)` | n/a | yes |
| task\_cpu\_architecture | The CPU architecture of the task that prepares the database: `ARM64` or `X86_64`. The image must be built for it; `db/Dockerfile` builds for both. | `string` | `"ARM64"` | no |
| task\_subnet\_ids | The IDs of the private subnets, where the task that prepares the database runs (`private_subnet_ids` of the module network). | `list(string)` | n/a | yes |
| vpc\_id | The ID of the VPC (`vpc_id` of the module network). | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| database\_name | The name of the database that every context shares. |
| db\_task | What `aws ecs run-task` needs to run the task that prepares the database, once at each deploy, before the services start: the task definition, the private subnets and the security group of its network configuration (awsvpc, without a public address), and the name of its container. |
| endpoint | The address of the writer of the cluster. |
| port | The port of PostgreSQL. |
| runtime\_secret\_arns | The ARN of the secret of the runtime role of each context, by context. The key `url` of the secret is the `DATABASE_URL` of the service of that context: its task definition reads it with `valueFrom = "<ARN>:url::"`, and its execution role needs `secretsmanager:GetSecretValue` on the ARN. |
| security\_group\_id | The ID of the security group of the cluster. A context lets its tasks connect with an ingress rule in this group, from the security group of its tasks, on the port of PostgreSQL. |
<!-- END_TF_DOCS -->
