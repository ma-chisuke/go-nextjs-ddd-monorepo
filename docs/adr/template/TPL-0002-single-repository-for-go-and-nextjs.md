---
status: accepted
date: 2026-11-17
---

# TPL-0002: Keep the Go API and the Next.js app in one repository

## Context and Problem Statement

The template is a web product made of two programs: an HTTP API written in Go, and a web app written with Next.js.
Around them are the API contracts, the database schema, the infrastructure code, and the documents.

A typical change crosses these parts.
To show a new field on a screen, a developer changes the OpenAPI document, the Go handler, the code generated from the document on both sides, and the screen itself.
If the parts live in separate repositories, that one change becomes several pull requests that must merge in a fixed order, with a published version of the contract between them.

The template also ships a harness for coding agents: the instructions in `AGENTS.md`, the hooks, and the checks behind `make check`.
An agent can check only what it has in its checkout.
If the API and the screens are in separate repositories, an agent changes one side and can neither build nor test the other.

Where should the code of the template live, and what should hold its parts together?

## Decision Drivers

- A contract change and the code on both sides of it land in one pull request, and the checks see all of it.
- People, agents, hooks, and CI use the same commands with the same tool versions.
- The native tools (`go` and `pnpm`) work as they are; a reader of the template does not have to learn a build system first.
- The layout holds while the repository grows, or there is a clear sign of when to change it.

## Considered Options

- One repository, driven by Make, a pnpm workspace, and one Go module
- Separate repositories for the API, the web app, the contracts, and the infrastructure
- One repository with a monorepo build system (Bazel, Nx, or Turborepo)

## Decision Outcome

Chosen option: "One repository, driven by Make, a pnpm workspace, and one Go module", because it puts every side of a change in one pull request and in front of one set of checks, and it adds no tool that the template does not need yet.

The top-level directories are as follows.

| Directory | Contents |
|---|---|
| `backend/` | The Go code of every bounded context, as one Go module |
| `frontend/` | The Next.js app (App Router), as a package of the pnpm workspace |
| `contracts/` | The OpenAPI documents of the HTTP APIs, and the schemas of the integration events |
| `db/` | What the database needs before any bounded context can use it: schemas, roles, and extensions |
| `infra/` | Terraform modules and Terragrunt configurations |
| `docs/` | Decision records, guides, and the documents of each bounded context |
| `tools/` | Pinned Go tools, and the scripts and checks that this repository writes itself |

- The Makefile is the single entry point. Its main targets are `make setup`, `make fmt`, `make gen`, `make check`, and `make check-all`. Hooks, agents, and CI run tools through `make` targets, not directly. The one exception is the `commit-msg` hook, which runs the script `tools/scripts/commit-msg.sh` on the message (TPL-0023).
- `tools/go.mod` is a separate Go module for the tooling of the repository: it pins Go tools with `tool` directives, and it holds the checks that this repository writes, such as `tools/cmd/docs`. The code of the product has one Go module, in `backend/`.
- Each bounded context still builds its own container image from `backend/cmd/<context>/`, and deploys on its own. Its API, and its worker if it has one, run from that image as separate services (TPL-0014).
- No monorepo build system is added until the checks are too slow even when they look only at the changed files. The time budget of `make check` (TPL-0023) is the sign. A team that reaches it records the change in a project record.

### Consequences

- Good, because a contract change, the Go code, the TypeScript code, and the generated code on both sides are reviewed and checked together.
- Good, because an agent works in one checkout: it reads the contract that it implements, and it runs every check that its change can break.
- Good, because the harness exists once: one `AGENTS.md`, one set of hooks, one CI, and one place where tool versions are pinned.
- Good, because a change to shared code, such as a lint rule or the technical platform under `backend/internal/platform/`, reaches every bounded context in the same commit.
- Bad, because the slow checks must be narrowed to the changed paths, or every pull request runs all of them over every file. `CHECK_SCOPE=changed` does that, and it widens to every file when a change touches a tool pin or the definition of a check (TPL-0023).
- Bad, because GitHub grants access per repository: everyone who can read one part can read all of it. `CODEOWNERS` can require a reviewer for each path, but it cannot hide a path.
- Bad, because the repository has one history and one sequence of release tags; its parts do not have versions of their own.
- Bad, because every bounded context uses the same version of each Go dependency, so an upgrade must work for all of them at once.

### Confirmation

Review enforces the layout: no check fails yet when a top-level directory is added or removed.

These parts exist now:

- `make check` and `make check-all` run every check of the repository. `lefthook.yml` calls the same targets, narrowed to the staged files before a commit (`CHECK_SCOPE=staged`), so a hook and a manual run with the same scope give the same result.
- `pnpm-workspace.yaml` lists `frontend` as the only package of the workspace. So far, `frontend/` holds the configuration of its checks only.
- The CI workflows (TPL-0018) call the same targets. The job that runs `make check` checks every file; the job that adds the slow checks of `make check-all` narrows them to the changed files on a pull request (`CHECK_SCOPE=changed`).
- `backend/` and `contracts/` hold the bounded contexts and their contracts, since the first bounded context.
- `infra/` holds the Terraform modules that the local environment shares with AWS, and `infra/local/`, which applies them to AWS Emulator (TPL-0015, TPL-0017), since the events milestone.

These parts arrive later:

- The Next.js app in `frontend/` arrives with its milestone, and the other Terraform modules and the Terragrunt configurations in `infra/` with the deployment.

## Pros and Cons of the Options

### One repository, driven by Make, a pnpm workspace, and one Go module

- Good, because the only tools in front are the ones that the code needs anyway, `make`, `go`, and `pnpm`, and mise, which installs them.
- Good, because one pull request holds every side of a change.
- Bad, because nothing computes which checks a change affects. The checks narrow themselves by path, which is coarser than a build graph.
- Bad, because the only build cache is the one that `go` and `pnpm` keep on each machine.

### Separate repositories for the API, the web app, the contracts, and the infrastructure

- Good, because access, ownership, and versions are set for each repository.
- Good, because each repository has a small and fast CI.
- Bad, because a contract change becomes several pull requests that merge in order, with a published version of the contract between them.
- Bad, because the harness is copied into each repository, and the copies drift apart.
- Bad, because an agent in one repository can neither see nor check the other side of its change.

### One repository with a monorepo build system (Bazel, Nx, or Turborepo)

- Good, because the build system knows which targets a change affects, and it can share a cache between machines.
- Good, because Bazel builds Go and TypeScript in a hermetic environment, so a result does not depend on the machine.
- Bad, because it adds a second description of the build: `BUILD` files for Bazel, or project files for Nx and Turborepo.
- Bad, because Nx and Turborepo are built around JavaScript packages, and Go needs plugins or wrapper scripts.
- Bad, because people and agents learn and run the build system instead of `go` and `pnpm`, and the template is not large enough yet to repay that.

## More Information

- [pnpm workspaces](https://pnpm.io/workspaces)
- [Go modules reference](https://go.dev/ref/mod)
- TPL-0003 describes how the code inside `backend/` and `frontend/` is split by bounded context.
