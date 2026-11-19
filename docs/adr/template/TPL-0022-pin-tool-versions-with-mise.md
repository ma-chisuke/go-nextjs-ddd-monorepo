---
status: accepted
date: 2026-11-19
---

# TPL-0022: Pin tool versions in one place per ecosystem, with mise at the root

## Context and Problem Statement

`make check` and `make check-all` run dozens of tools: runtimes, package managers, formatters, linters, and scanners.
People run them on macOS and Linux, coding agents run them in their own sandboxes, and CI runs them on fresh machines.
When two of these places run different versions of a tool, a check passes in one place and fails in another, and the cause is hard to see.

A version number alone is not enough either.
A tool is code that runs with access to the repository, and in CI with access to tokens.
If a release is replaced after the fact, or a download is tampered with, every machine that installs the tool runs the attacker's code.

How do we make every machine install the same tools, byte for byte, without asking anyone to install them by hand?

## Decision Drivers

- People, agents, and CI run the same version of every tool.
- An install fails when a download differs from the one that was locked.
- One command, `make setup`, installs everything.
- Nothing is installed globally, so two projects on one machine do not interfere.
- The dependencies of tools stay out of the dependencies of the product.
- Each ecosystem keeps the lockfile that its own tools and Renovate understand.

## Considered Options

- mise at the root, with each ecosystem's own lockfile for its packages
- aqua alone
- asdf
- Nix or devbox
- A container image as the only environment
- A list of tools and versions in the README

## Decision Outcome

Chosen option: "mise at the root, with each ecosystem's own lockfile for its packages", because it pins runtimes and standalone binaries in one file with their checksums, and leaves packages to the lockfiles that already verify them.

The rules are as follows.

- `mise.toml` pins the exact version of each runtime (Go, Node.js, Python), each package manager (pnpm, uv), and each standalone binary.
- `mise.lock` records the URL and the checksum of every tool for four platforms: Linux and macOS, each on x64 and arm64. Every tool that the template pins publishes a build for all four. `locked = true` makes an install fail unless the lockfile describes it.
- Where a publisher provides provenance (GitHub artifact attestations, cosign, or SLSA), mise verifies it when it writes the lock, and records which kind it verified.
- A tool that is a package of an ecosystem is pinned by the lockfile of that ecosystem: npm tools in `package.json` and `pnpm-lock.yaml`, Python tools in `pyproject.toml` and `uv.lock` (with hashes), and Go tools as `tool` directives in `tools/go.mod`.
- The Makefile puts all of these tools on `PATH`. Nobody installs a tool globally.
- `GOTOOLCHAIN=local` stops Go from downloading a toolchain other than the one that mise installed.
- After a change to `mise.toml`, `make tools-lock` writes `mise.lock` again.
- The tools of a deployment are pinned there too: Terraform, Terragrunt, and the command-line tool of AWS, `aws-cli` (2.41.0 in the infrastructure milestone). `tools/scripts/infra.sh` and `tools/scripts/images.sh` call it to push the images and to run the task that prepares the database, and the rehearsal calls it to request a certificate and create a hosted zone in AWS Emulator. The workflows `Plan` and `Deploy` install these tools, and `jq`, from `mise.toml` and nothing else.

Two parts of this decision differ from the first plan, on purpose.

- Go tools live in a separate module, `tools/`, not in the `go.mod` of the backend. The first plan put them in the backend's module. A separate module keeps the dependencies of the tools out of the backend's module graph and out of its SBOM, and the license scan skips `tools/` (`scan-deps`, TPL-0024), so the product declares only what it runs.
- npm and Python tools are not installed through the `npm:` and `pipx:` backends of mise. Those backends pin the tool itself but do not lock its transitive dependencies, while pnpm and uv lock every transitive dependency with a hash.

### Consequences

- Good, because one command installs the same tools, byte for byte, on every supported machine.
- Good, because a replaced release or a tampered download fails the install: its checksum differs from the one in `mise.lock`.
- Good, because the dependency graph, the SBOM, and the license scan of the product contain only what the product uses.
- Bad, because there are four kinds of manifest (`mise.toml`, `package.json`, `pyproject.toml`, `tools/go.mod`), each with its own lockfile and its own commands.
- Bad, because mise itself must be installed by hand first, and the repository must be trusted with `mise trust`, which `make setup` runs.
- Bad, because `mise.lock` holds four entries per tool, so every update changes many lines.
- Bad, because the lock covers Linux and macOS only. On Windows, people use WSL or the dev container.
- Bad, because adding a tool takes more than one install command: an entry in `mise.toml`, then `make tools-lock`.

### Confirmation

- `make setup` runs `mise install` under `locked = true`, `pnpm install --frozen-lockfile`, and `uv sync --locked`. Each fails when its lockfile does not match its manifest. `make setup` also downloads the Go modules and fills the Go build cache, so `make check` needs no network afterward (TPL-0023).
- The Makefile sets `GOFLAGS=-mod=readonly`, so a Go command fails instead of changing `go.mod` or `go.sum`, and `GOTOOLCHAIN=local`.
- CI runs the same `make setup` with the same files.
- `make check-all` validates `mise.toml` against the schema that mise publishes (target `lint-schema-remote`).
- Review confirms that no tool is installed globally. The instructions for agents say so as well.

## Pros and Cons of the Options

### mise at the root, with each ecosystem's own lockfile

- Good, because one file pins runtimes, package managers, and binaries, and mise installs most binaries from the aqua registry with checksums.
- Good, because each ecosystem keeps the lockfile that its tools and Renovate understand.
- Bad, because four manifests must be kept in step.

### aqua alone

- Good, because it verifies checksums and provenance, and its registry covers most standalone command-line tools.
- Bad, because it is built for standalone command-line tools; runtimes such as Python, with their own installers and environments, are outside its focus, so a second manager would remain.

### asdf

- Good, because it is widely known, and its `.tool-versions` file is simple.
- Bad, because each plugin is a script by its own author that decides for itself whether to verify a download, and asdf keeps no lockfile of checksums.

### Nix or devbox

- Good, because every input is pinned by its hash, and the environment can be reproduced exactly.
- Bad, because Nix is a large install and a language of its own that every contributor and every agent sandbox would need. devbox hides Nix but still needs it underneath.

### A container image as the only environment

- Good, because one image holds every tool, and every machine that runs it has the same set.
- Bad, because editors and agents that run on the host would not see the tools, and every command would pay the cost of a container.
- Bad, because the image is one more artifact to build, pin, scan, and update.

### A list of tools and versions in the README

- Good, because there is nothing to install or maintain beyond the list.
- Bad, because versions drift from machine to machine, nothing verifies a download, and an agent installs whatever version it finds.

## More Information

- [mise](https://mise.jdx.dev/) and [mise.lock](https://mise.jdx.dev/dev-tools/mise-lock.html)
- The defenses of the supply chain that build on these pins are in TPL-0024, and the updates of the pins in TPL-0025.
