---
status: accepted
date: 2026-11-19
---

# TPL-0015: Manage the infrastructure with Terraform and Terragrunt, with stg and prd in separate AWS accounts

## Context and Problem Statement

The template runs on AWS: a network, an Aurora cluster (TPL-0016), an ECS cluster whose services call each other through ECS Service Connect (TPL-0014), an EventBridge bus (TPL-0013), a service for each context, and Next.js behind one public load balancer.
It is built twice, for two environments, stg and prd.

The infrastructure must be code, reviewed and planned in pull requests like the rest of the repository.
Three needs pull in different directions.

- The environments must have the same shape, so that what works in stg works in prd, but each environment has its own values, such as sizes, capacity, and domain names.
- A change to one context must plan and apply only that context. One state for everything makes every plan slow and every apply risky.
- A mistake in stg must not reach prd, and credentials for one environment must not work in the other.

Which tools describe the infrastructure, how is the code split, and how are the environments kept apart?

## Decision Drivers

- stg and prd have the same shape, written once, with different values.
- A change to one context plans and applies only the state of that context.
- A mistake or a leaked credential in stg cannot reach prd.
- The tools are widely known, and linters and scanners exist for them.
- The versions of the tools are pinned in one place (TPL-0022).

## Considered Options

- Terraform modules instantiated by Terragrunt, with one AWS account for each environment
- Terraform alone, with workspaces or a variable file for each environment
- OpenTofu in place of Terraform
- AWS CDK
- Pulumi

## Decision Outcome

Chosen option: "Terraform modules instantiated by Terragrunt, with one AWS account for each environment", because Terragrunt writes the state and provider configuration once for many small units, and separate accounts are the strongest boundary that AWS offers between stg and prd.

The rules are as follows.

- Terraform modules live in `infra/modules/`: `network` (the VPC, its subnets, and the endpoints of the AWS services), `registry` (the image repositories), `database`, `platform` (the ECS cluster, the namespace of Service Connect, the EventBridge bus, and the security group of the services), `context-service`, `web` (Next.js and the public load balancer), and `github-oidc` (the roles of GitHub Actions, TPL-0018).
- Two smaller modules hold the parts of `platform` and `context-service` that the local environment shares with AWS: `event-bus`, the EventBridge bus of `platform`, and `event-subscription`, the queue, the dead-letter queue, and the rule of one consuming context (TPL-0013). The root `infra/local/` applies them to AWS Emulator (TPL-0017). It is not a unit: it accepts only an endpoint on the local machine, and its state lives outside the repository and is thrown away when the emulator restarts.
- A Terragrunt tree, `infra/live/{stg,prd}/`, instantiates the modules. Each directory with a `terragrunt.hcl` is a unit with its own state: `ci`, `network`, `registry`, `database`, `platform`, `web`, and one unit for each context under `contexts/<context>/`.
- `infra/live/<environment>/<unit>/terragrunt.hcl` is the same file in every unit of every environment. What a unit creates is in `infra/live/_units/<unit>.hcl`, which names its module and its inputs; what an environment is, in `infra/live/<environment>/env.hcl`; and what all environments share, in `infra/live/system.hcl`: the name of the system, the repository on GitHub, and the list of contexts.
- `infra/live/root.hcl` generates three files for every unit: the S3 backend of its state, the AWS provider with `allowed_account_ids`, which refuses every account but the one of the environment, and the versions of the providers, from `infra/live/providers.hcl`.
- One lock file, `infra/live/providers.lock.hcl`, holds the checksums of the providers for every unit. Each unit initializes with `-lockfile=readonly`, no unit keeps a lock file of its own, and `make infra-lock` writes the file after a change to `providers.hcl`.
- State is stored in S3 with the native locking of S3 (`use_lockfile`), one state for each unit, in a bucket of each account. `make infra-bootstrap` creates the bucket and applies the unit `ci`, once for each account, run by a person with administrator access, because the workflows cannot assume roles that do not exist yet.
- Units read the outputs of other units through Terragrunt dependencies. For example, a context unit reads the outputs of `network`, `registry`, `database`, and `platform`. Until a unit has a state, the units that depend on it see the mock outputs of `infra/live/mocks.hcl`, in `terraform validate` and in a plan, never in an apply.
- stg and prd are separate AWS accounts. Their trees have the same shape, and only the values in `env.hcl` differ. The values that the template ships are placeholders, such as the accounts `111111111111` and `222222222222` and the domain `example.com`; a team replaces them before the first apply ([Deploy to AWS](../../guides/deploy.md)).
- Terragrunt keeps its working copies of the units in `.cache/terragrunt/<mode>/<environment>/.terragrunt-cache`, outside `infra/live/`. Terragrunt 1.2.2 looks for units in the whole repository and skips only directories of that name, so under any other name it would take its own copies for units.
- The versions of Terraform and Terragrunt are pinned in `mise.toml` (TPL-0022).
- CI signs in to each account through OIDC, with no long-lived keys (TPL-0018). The workflow `Plan` plans every unit of stg and of prd on a pull request, and the workflow `Deploy` applies them after a merge.

Three parts of this decision differ from the first plan.

- The first plan gave each unit directory its own configuration. Every `terragrunt.hcl` is now the same file, and what a unit creates lives once in `_units/`, so that stg and prd cannot drift apart: a change to a unit is a change to both environments, and `lint-infra-live` fails when the trees differ.
- The first plan named five modules and five kinds of unit. The milestone added the module `registry`, because the images need a repository before a deployment can push them, and the module `github-oidc` with the unit `ci`, because the roles of GitHub Actions belong to each account too.
- The first plan said that CI plans every unit on each pull request that changes `infra/`. The workflow `Plan` does, also when a pull request changes a context, the frontend, or the database, because those change the tags of the images and so the task definitions. Its jobs are skipped until the repository variables with the roles are set.

### Consequences

- Good, because a change to one context plans and applies one small state: plans are fast, and an apply cannot change another context.
- Good, because the shape is written once in the modules, so stg runs the same code as prd.
- Good, because separate accounts isolate the environments: permissions in stg grant nothing in prd, and quotas and costs are separate.
- Good, because Terraform has the largest ecosystem of providers and modules, with a linter (tflint) and scanners (Trivy) that know it.
- Bad, because there are two tools to learn and pin, and Terragrunt adds a layer of configuration on top of Terraform.
- Bad, because many small states depend on one another. A change to an output of `network`, for example, must be applied before the units that read it, in order.
- Good, because stg and prd cannot drift apart: they have the same units, every `terragrunt.hcl` is the same file, and every `env.hcl` sets the same keys.
- Good, because the credentials of one account cannot change the other, whatever directory a command runs in: the provider of each unit accepts only the account of its environment.
- Bad, because each account needs its own setup: a bucket for the state, roles for OIDC, a hosted zone in Amazon Route 53, and a verified identity of Amazon SES. A person with administrator access creates the bucket and the roles, outside the workflows.
- Bad, because the definition of a unit is not in its directory. A reader learns to look in `_units/`, and the path of the include in `terragrunt.hcl` is computed rather than written out.
- Bad, because nothing has been applied to AWS. The tests of the modules plan against a mock of the provider, `validate-live` mocks the outputs of other units, and the rehearsal applies to an emulator, so what only AWS can show is checked at the first apply ([Deploy to AWS](../../guides/deploy.md#what-the-first-apply-must-confirm)).
- Bad, because Terraform is under the Business Source License, which is not an open source license. The license does not restrict a team that uses Terraform to manage its own infrastructure.

### Confirmation

- `make check` runs `fmt-check-terraform` (`terraform fmt` on the files of the Terraform language), `fmt-check-terragrunt` (`terragrunt hcl fmt` on every other HCL file, such as `root.hcl` and `terragrunt.hcl`), `lint-terraform` (tflint with the AWS ruleset), `lint-terraform-docs` (whether the module READMEs that terraform-docs writes are up to date), and `scan-config` (Trivy, for misconfiguration).
- `make setup` installs the tflint plugin that `.tflint.hcl` pins, so these checks need no network.
- `make check` runs `lint-infra-live`, which fails in any of these cases:
  - the environments do not have the same units, or a `terragrunt.hcl` differs from the others;
  - a unit has no definition in `_units/`, or a definition has no unit;
  - an `env.hcl` lacks a key that another sets;
  - the contexts of `system.hcl`, the commands of `backend/cmd/`, and the units under `contexts/` differ.
- `make check-all` adds `validate-terraform`, which runs `terraform validate` and then `terraform test` on the `tests/*.tftest.hcl` of each module, against `mock_provider "aws"`. It runs `terraform init` and downloads the providers.
- `make check-all` adds `validate-live`, which runs `terraform validate` in every unit, as Terragrunt builds it, with the outputs of other units mocked. One environment stands for all, because `lint-infra-live` proves that they have the same units.
- `make check-all` adds `rehearse-infra`, which applies the units of stg to AWS Emulator in the order of a deployment, nine units in about three minutes, and then plans the units of prd against them (TPL-0017). A module that does not apply, a unit that reads an output that another unit does not have, or a value of an `env.hcl` that a module rejects fails there.
- `make infra-lock` writes `providers.lock.hcl` for the four platforms of `mise.lock`, and every Terragrunt run fails on a provider that the lock file does not name.
- The tools are pinned in `mise.toml` (TPL-0022). These checks cover every module, the units in `infra/live/`, and `infra/local/`.
- `infra/local/.terraform.lock.hcl` locks the AWS provider with its hashes for four platforms, and `tools/scripts/aws.sh` initializes with `-lockfile=readonly`, so a provider that the lock does not name fails to install.
- `make smoke`, part of `make check-all`, applies `infra/local/` to AWS Emulator and sends events through the bus and the queue that it creates, so a module that does not apply, or that does not deliver, fails there (TPL-0017).
- The workflow `Plan` runs `make infra-plan` for stg and for prd, as the read-only role of each account, and its log shows what a merge would change, unit by unit.

## Pros and Cons of the Options

### Terraform modules instantiated by Terragrunt, with one AWS account for each environment

- Good, because the configuration of the state and the providers is written once, each unit has a small state, units declare their dependencies, and both environments have the same shape.
- Bad, because there are two tools, a layer of configuration to learn, and an order to respect between units.

### Terraform alone, with workspaces or a variable file for each environment

- Good, because there is one tool, and nothing beyond what the documentation of Terraform covers.
- Bad, because the workspaces of one directory share one backend, which fits poorly with separate accounts. The documentation of Terraform itself says that workspaces are not a suitable isolation mechanism when deployments need separate credentials and access controls.
- Bad, because splitting the infrastructure into many states means repeating the backend and provider blocks in every root module, and passing outputs between states by hand.

### OpenTofu in place of Terraform

- Good, because it is an open source (MPL 2.0) fork of Terraform under the Linux Foundation, compatible with most Terraform code, and supported by Terragrunt. It also adds features of its own, such as encryption of the state.
- Bad, because it diverges from Terraform release by release, and Terraform still has the larger user base, so a team that copies the template is more likely to know Terraform.

### AWS CDK

- Good, because the infrastructure is written in a general-purpose language such as TypeScript or Go, with types and loops, and CloudFormation keeps the state.
- Bad, because what deploys is a CloudFormation template generated from code, so a reviewer reads code that generates the template, and tools chosen for Terraform, such as tflint and terraform-docs, do not apply.
- Bad, because it works only on AWS, and every change goes through a CloudFormation stack, which can need manual recovery when an update fails.

### Pulumi

- Good, because the infrastructure is written in a general-purpose language, the engine is open source (Apache 2.0), and it offers many providers.
- Bad, because by default the state lives in Pulumi's hosted service, which needs an account, and the template must work without one. A team can keep the state in S3 instead, as with Terraform.
- Bad, because it is less widely known than Terraform, and infrastructure written as a program has the same cost in review as with CDK.

## More Information

- The design notes of the template named Terragrunt 1.0. The template began with the latest 1.x release on the day the work started, and Renovate keeps it up to date (TPL-0025). The infrastructure milestone was built with Terragrunt 1.2.2 and Terraform 1.17.2.
- [Deploy to AWS](../../guides/deploy.md) prepares an account, deploys to it, and lists what the first apply must confirm.
- A team that prefers OpenTofu can switch with a project record. The modules stay in HCL, and Terragrunt supports both.
- [Terragrunt documentation](https://terragrunt.com/)
- [The S3 backend of Terraform](https://developer.hashicorp.com/terraform/language/backend/s3), including `use_lockfile`
