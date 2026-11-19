# Deploy to AWS

This guide prepares an AWS account for an environment, deploys to it, and explains what the workflows do afterwards.
The decisions behind it are in TPL-0014 to TPL-0018; the map is in [ARCHITECTURE.md](../../ARCHITECTURE.md).

The template was built without an AWS account.
Everything here passes the checks of [Without an AWS account](#without-an-aws-account), and nothing was applied to AWS yet.
[What the first apply must confirm](#what-the-first-apply-must-confirm) lists what those checks cannot prove.

## What an environment is

stg and prd are two AWS accounts with the same units, in `infra/live/<environment>/`.
A unit is a directory with a `terragrunt.hcl`; it has one Terraform state, and Terragrunt applies it with the module that `infra/live/_units/` names for it.

| Unit | Module | What it creates |
|---|---|---|
| `ci` | `github-oidc` | The roles that GitHub Actions assumes |
| `network` | `network` | The VPC, its subnets, and the endpoints of the AWS services |
| `registry` | `registry` | The image repositories |
| `database` | `database` | The Aurora cluster, the secrets of the roles of each context, and the task that prepares the database |
| `platform` | `platform` | The ECS cluster, the Service Connect namespace, the event bus, and the security group of the services |
| `contexts/<context>` | `context-service` | The services, the roles, the logs, and the event subscription of one context |
| `web` | `web` | The server of Next.js and the public load balancer |

- What differs between the environments is in `infra/live/<environment>/env.hcl`, and nowhere else. Every `terragrunt.hcl` is the same file, and `make check` fails when the two trees differ (`lint-infra-live`).
- What all environments share is in `infra/live/system.hcl`: the name of the system, the repository on GitHub, and the list of contexts.
- The versions of the providers are in `infra/live/providers.hcl`, with their checksums in `providers.lock.hcl`. After a change to the versions, run `make infra-lock`.

## Prepare an account

Do this once for stg, and once for prd, with the name of the environment in place of `stg`.

1. **Fill in the environment.** In `infra/live/stg/env.hcl`, replace the placeholders: the account ID, the Region and its Availability Zones, the domain name of the site and the ID of its public hosted zone in Amazon Route 53, and the address and the identity that the mail comes from. In `infra/live/system.hcl`, set the name of the system, which starts the name of the state bucket, and the repository.
2. **Verify the mail identity.** In Amazon SES, in the Region of the environment, verify the domain of `mail_identity`, and ask for production access. In the sandbox, SES sends only to verified addresses.
3. **Create the state bucket and the roles.** With credentials that have administrator access to the account:

   ```sh
   make infra-bootstrap ENV=stg
   ```

   It creates the S3 bucket of the Terraform state, then applies the unit `ci` and prints the ARNs of the two roles, `github-plan` and `github-deploy`. Terraform asks before it applies.
4. **Tell GitHub.** In the settings of the repository:
   - Create the Environment `stg`, and allow deployments from `main` only. For `prd`, also require a reviewer: the job of prd then waits for that person.
   - Set the repository variables `AWS_REGION`, `AWS_PLAN_ROLE_ARN_STG`, and `AWS_DEPLOY_ROLE_ARN_STG` (for prd, the two variables that end in `_PRD`). They are not secrets. Until the variable of a role is set, the job that needs it is skipped.

## Deploy

After the preparation, a merge into `main` deploys: the workflow `Deploy` runs `make deploy ENV=stg`, and then, after the approval, `make deploy ENV=prd`.
You can run the first deployment of stg from your machine instead, with Docker and with credentials of the account. `make deploy ENV=prd` refuses to run outside the workflow.

A deployment does four things, in this order (`tools/scripts/infra.sh`):

1. It applies the unit `registry`, then builds and pushes each image whose tag the registry does not hold (`tools/scripts/images.sh`).
2. It applies the units `network`, `database`, and `platform`.
3. It runs the task that prepares the database, and waits for it: `db/bootstrap.sql`, the passwords of the roles, and the `schema.sql` of every context with pgschema, as `make db-up` does locally.
4. It applies every unit.

The tag of an image is a hash of the files that the image is built from.
A change to one context therefore changes the task definition of that context alone, and step 4 restarts only its services; the other contexts keep running.
A service whose new tasks do not become healthy is rolled back by ECS (the deployment circuit breaker).

Step 3 changes the schema before step 4 starts the new code, while the old code still runs.
A schema change must therefore work with both: add first, and remove in a later deployment ([change a schema](change-a-schema.md)).

## Read the plan of a pull request

The workflow `Plan` runs `make infra-plan` for stg and for prd on a pull request that changes the infrastructure, a context, the frontend, or the database.
The log of each job shows what a merge would change in that environment, unit by unit.
A job signs in as `github-plan`, which can read and cannot change.
A pull request from a fork gets no plan: GitHub gives it no token.

## Change an environment

| To change | Edit | Then |
|---|---|---|
| A value of one environment, such as the capacity of the database | `infra/live/<environment>/env.hcl` | Every `env.hcl` sets the same keys |
| What a unit creates, in every environment | `infra/live/_units/<unit>.hcl`, or its module in `infra/modules/` | `make gen` after a change to a module |
| The units of a new context | `infra/live/_units/contexts/<context>.hcl`, the same `terragrunt.hcl` in each environment, and `contexts` in `system.hcl` | [Add a context](add-a-context.md) |
| The version of a provider | `infra/live/providers.hcl` | `make infra-lock` |
| What the tasks may reach outside AWS | `nat_gateways` of the unit `network`, and a rule in the security group of the context | See the README of `infra/modules/network` |

## Without an AWS account

Three checks run the environments without an account, and CI runs them on every pull request that touches the infrastructure.

- `make validate-terraform` validates each module and runs its tests (`tests/*.tftest.hcl`), which plan against a mock of the AWS provider.
- `make validate-live` runs `terraform validate` in every unit, as Terragrunt builds it, with the outputs of the other units mocked (`infra/live/mocks.hcl`). One environment stands for all, because they have the same units. It needs no Docker.
- `make rehearse-infra` applies the units of stg to AWS Emulator, in the order of a deployment, and plans the units of prd against it. A module that does not apply, a unit that reads an output that another unit does not have, or a value of an `env.hcl` that a module rejects fails here.

The rehearsal is not a deployment. The emulator keeps what Terraform creates and starts no task, so it pushes no image and prepares no database, and three inputs differ from AWS: the emulator knows Aurora PostgreSQL up to version 17, it lacks the endpoint of the API of Amazon SES, and it never issues a certificate, so the rehearsal requests one itself and hands it to the unit `web`.
`make test-db-image` covers the task that prepares the database: it builds the image and runs it against the local PostgreSQL.

## What the first apply must confirm

- **The start of a task.** Pulling the image through the endpoints, reading the secret of the database, and resolving `http://facility:8080` through Service Connect.
- **The endpoint of the API of Amazon SES.** Its service name, `email`, follows the documentation of SES, which does not spell the name out ([README of the module](../../infra/modules/network/README.md)).
- **The subjects of the tokens.** Whether AWS accepts the subject of a pull request and of an Environment in the form that the roles expect ([README of the module](../../infra/modules/github-oidc/README.md)).
- **The plan with the read-only role.** Whether `ReadOnlyAccess` is enough to refresh every resource, the versions of the secrets in particular.
- **Aurora.** The engine version, the pause at 0 ACU, and the time that the task that prepares the database waits while the cluster resumes.
- **The certificate.** The validation through the hosted zone, which no check runs.
