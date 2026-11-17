---
status: accepted
date: 2026-11-17
---

# TPL-0015: Manage the infrastructure with Terraform and Terragrunt, with stg and prd in separate AWS accounts

## Context and Problem Statement

The template runs on AWS: a network, an Aurora cluster (TPL-0016), an ECS cluster with ECS Gateway (TPL-0014), an EventBridge bus (TPL-0013), a service for each context, and Next.js.
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

- Terraform modules live in `infra/modules/`: `network`, `database`, `platform` (the ECS cluster, ECS Gateway, and the EventBridge bus), `context-service`, and `web`.
- A Terragrunt tree, `infra/live/{stg,prd}/`, instantiates the modules. Each directory that instantiates a module is a unit with its own state: `network`, `database`, `platform`, `web`, and one unit for each context under `contexts/<context>/`.
- `infra/live/root.hcl` generates the configuration of the state backend and of the providers for every unit.
- State is stored in S3 with the native locking of S3 (`use_lockfile`), one state for each unit.
- Units read the outputs of other units through Terragrunt dependencies. For example, a context unit reads the outputs of `network`, `database`, and `platform`.
- stg and prd are separate AWS accounts. Their trees have the same shape, and only the values in `env.hcl` differ.
- The versions of Terraform and Terragrunt are pinned in `mise.toml` (TPL-0022).
- CI signs in to each account through OIDC, with no long-lived keys (TPL-0018).

### Consequences

- Good, because a change to one context plans and applies one small state: plans are fast, and an apply cannot change another context.
- Good, because the shape is written once in the modules, so stg runs the same code as prd.
- Good, because separate accounts isolate the environments: permissions in stg grant nothing in prd, and quotas and costs are separate.
- Good, because Terraform has the largest ecosystem of providers and modules, with a linter (tflint) and scanners (Trivy) that know it.
- Bad, because there are two tools to learn and pin, and Terragrunt adds a layer of configuration on top of Terraform.
- Bad, because many small states depend on one another. A change to an output of `network`, for example, must be applied before the units that read it, in order.
- Bad, because each account needs its own setup: a bucket for the state, roles for OIDC, and DNS.
- Bad, because Terraform is under the Business Source License, which is not an open source license. The license does not restrict a team that uses Terraform to manage its own infrastructure.

### Confirmation

- `make check` runs `fmt-check-terraform` (`terraform fmt` on the files of the Terraform language), `fmt-check-terragrunt` (`terragrunt hcl fmt` on every other HCL file, such as `root.hcl` and `terragrunt.hcl`), `lint-terraform` (tflint with the AWS ruleset), `lint-terraform-docs` (whether the module READMEs that terraform-docs writes are up to date), and `scan-config` (Trivy, for misconfiguration).
- `make setup` installs the tflint plugin that `.tflint.hcl` pins, so these checks need no network.
- `make check-all` adds `validate-terraform` (`terraform validate`), which runs `terraform init` and downloads the providers.
- The tools are pinned in `mise.toml` (TPL-0022). M0 has no `.tf` file, so these checks have nothing to check yet; they act when the first `.tf` file arrives.

With the infrastructure milestone, CI plans every unit (`terragrunt run --all plan`) on each pull request that changes `infra/`.

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

- The design notes of the template named Terragrunt 1.0. The template began with the latest 1.x release on the day the work started, and Renovate keeps it up to date (TPL-0025).
- A team that prefers OpenTofu can switch with a project record. The modules stay in HCL, and Terragrunt supports both.
- [Terragrunt documentation](https://terragrunt.com/)
- [The S3 backend of Terraform](https://developer.hashicorp.com/terraform/language/backend/s3), including `use_lockfile`
