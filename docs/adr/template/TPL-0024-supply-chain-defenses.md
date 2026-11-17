---
status: accepted
date: 2026-11-17
---

# TPL-0024: Defend the supply chain in layers

## Context and Problem Statement

Most of the code that runs in this repository was written by someone else: actions in CI, tools on every machine, npm, Python, and Go packages, and later the base images of containers.
Each of them runs with access to the source code, and in CI with access to tokens.
An attacker who takes over one of them reaches every repository that uses it.

Such an attack has already hit a tool that this template uses.
In March 2026, an attack (CVE-2026-33634) tampered with Trivy v0.69.4 and rewrote tags of the GitHub Actions `trivy-action` and `setup-trivy`.

A template multiplies the effect: what the template trusts, every copy of it trusts.

How do we limit what a compromised dependency can do, and how do we notice one?

## Decision Drivers

- A change in something that this repository depends on reaches it only through a change to this repository that someone can review.
- A tampered download fails to install.
- A new release waits before it is installed, so that a malicious release can be found and pulled first.
- Wherever possible, `make check`, `make check-all`, a package manager, or GitHub enforces a defense, rather than the care of a person.
- The defenses need no paid service.

## Considered Options

- Layered defenses with the tools that are already pinned
- Trust upstream tags and the latest versions
- Vendor every dependency into the repository
- A commercial platform for supply chain security

## Decision Outcome

Chosen option: "Layered defenses with the tools that are already pinned", because each layer stops a different kind of attack, and together they need no paid service and add little time to a check.

For GitHub Actions, pinact, zizmor, and ghalint enforce these rules in `make check` (TPL-0018).

- Every action is pinned to a full commit SHA, so a moved tag changes nothing.
- `permissions` are read-only by default, and each job asks for what it needs.
- Every job sets `timeout-minutes`.

For tools, mise enforces these rules when `make setup` installs them (TPL-0022).

- mise installs only what `mise.lock` describes: the URL and the checksum for each platform. Where the publisher provides provenance (GitHub artifact attestations, cosign, or SLSA), mise verifies it when it writes the lock.
- mise installs Trivy with a pinned version and checksum. The actions `trivy-action` and `setup-trivy` are not used, because of the attack described in this record.
- `make check` runs Trivy with the checks for misconfiguration that are built into the pinned binary (`--skip-check-update`). Otherwise Trivy downloads its newest checks when it runs: an unpinned download during the check, with results that change from day to day.

For npm packages, pnpm enforces these settings from `pnpm-workspace.yaml`.

- A new release cannot be installed until it is three days old (`minimumReleaseAge`).
- Versions are saved exactly, without ranges (`saveExact`).
- A transitive dependency from an untrusted source, such as a Git repository or a tarball URL, is refused (`blockExoticSubdeps`).
- A release whose publisher trust is weaker than in an earlier release is refused (`trustPolicy: no-downgrade`).
- No dependency runs a build script unless `allowBuilds` lists it (`strictDepBuilds`).

For Python packages, uv enforces these settings from `pyproject.toml`.

- `uv.lock` holds a hash for every package, and `uv sync --locked` fails when the lock does not match.
- Releases younger than three days are ignored (`exclude-newer`).
- uv uses the Python that mise installed, and never downloads another one (`python-downloads = "never"`).

For Go modules, the Go toolchain and the Makefile enforce these rules.

- `go.sum` holds a checksum for every module.
- `GOFLAGS=-mod=readonly` makes a command fail instead of changing `go.mod` or `go.sum`, and `GOTOOLCHAIN=local` stops Go from downloading another toolchain.
- govulncheck reports known vulnerabilities in the code that is actually called (`vuln-go` in `make check-all`).

Across the lockfiles, Trivy looks for known vulnerabilities and for dependencies under forbidden licenses (`scan-deps` in `make check-all`, and TPL-0019). The license scan covers only what ships with the product, so it leaves out development dependencies and the `tools/` module. Trivy is the one scanner for misconfiguration, vulnerabilities, licenses, and SBOMs: one pinned binary covers all four, so there is no second scanner to pin and to watch. For Go, govulncheck narrows the vulnerabilities to the code that is actually called (TPL-0023).

For secrets, two tools look for a credential in the code.

- gitleaks scans the staged changes before a commit, and the working tree in `make check` (`lint-secrets`).
- GitHub secret scanning and push protection check every push.

For the repository, the template uses these services on GitHub.

- CodeQL analyzes the Go and TypeScript code (`.github/workflows/codeql.yml`).
- OpenSSF Scorecard rates the security practices of the repository (`.github/workflows/scorecard.yml`).
- Private vulnerability reporting lets people report a vulnerability without making it public.
- Dependabot raises alerts for vulnerable dependencies. It opens no pull requests: updates come from Renovate (TPL-0025).

For containers, which arrive with the code and the infrastructure, these rules apply.

- Base images are minimal and pinned by digest.
- Containers run as a user other than root.
- Trivy scans every image and writes its SBOM.
- Builds get provenance through GitHub artifact attestations.

Two parts of this decision differ from the first plan, on purpose.

- The first plan avoided `trivy-action` only. The same attack rewrote the tags of `setup-trivy`, so neither action is used.
- The first plan named Trivy or syft for SBOMs. Trivy writes them, because it already scans the images, and each concern has one tool (TPL-0023).

### Consequences

- Good, because a moved tag, a replaced release, or a tampered download fails at install time instead of running.
- Good, because a malicious npm or Python release that is found and pulled within three days never reaches this repository.
- Good, because a compromised step in CI holds only the permissions that its job asked for, and only for as long as the job runs, which its timeout limits.
- Bad, because updates, security fixes included, arrive three days late unless someone overrides the wait.
- Bad, because adding a tool has friction: an entry in `mise.toml`, a lock for four platforms, and sometimes an entry in `allowBuilds`.
- Bad, because some tools publish no provenance. For them, `mise.lock` records only a checksum (gitleaks, shellcheck, and Vale among them). A checksum shows that every machine gets the file that was locked, not that the file is what its author built.
- Bad, because the repository sets no wait for Go modules or for the tools in `mise.toml`. A version that a person adds by hand, with `go get` or in `mise.toml`, arrives at once, and only the updates that Renovate proposes wait (TPL-0025).
- Bad, because several defenses on GitHub are settings, not files. `make github-settings` compares them with `docs/guides/github-settings.md`, but `make check` cannot see them.

### Confirmation

- `make check` runs `lint-actions-pins`, `lint-actions-security`, and `lint-actions-policy` on the workflows, `lint-secrets` on the files, and `scan-config` with the checks built into Trivy.
- `make setup` runs `mise install` under `locked = true`, `pnpm install --frozen-lockfile` with the settings in `pnpm-workspace.yaml`, and `uv sync --locked`. CI runs the same target.
- `make check-all` runs `scan-deps` and `vuln-go`.
- Review confirms that no workflow uses `trivy-action` or `setup-trivy`. No check enforces it.
- CodeQL and Scorecard run as workflows. Secret scanning, push protection, private vulnerability reporting, and Dependabot alerts are settings of the repository: `make github-settings` turns them on, and reports any that differ from `docs/guides/github-settings.md`.
- For containers, `make check` runs `lint-image-pins`, which fails on a base image without a digest, and `scan-config`, whose Trivy checks fail a Dockerfile whose image runs as root. The images of the services (`backend/Dockerfile`) run as the user `nonroot` of distroless static.
- `make check-all` runs `scan-images`, which scans the image of every service with Trivy for known vulnerabilities. `make sbom` writes a CycloneDX SBOM of each image to `dist/sbom/`.
- Provenance through GitHub artifact attestations arrives with the workflow that publishes the images, in the infrastructure milestone. That workflow attests the SBOMs of `make sbom` too.

## Pros and Cons of the Options

### Layered defenses with the tools that are already pinned

- Good, because a check, a package manager, or mise enforces most layers, and the rest are settings on GitHub that `make github-settings` and review confirm.
- Good, because it needs no paid service.
- Bad, because the settings are spread over many files, and each must be understood before it is changed.

### Trust upstream tags and the latest versions

- Good, because updates arrive at once, with no work.
- Bad, because a moved tag or a malicious release reaches every copy of the template on its next run.

### Vendor every dependency into the repository

- Good, because nothing is downloaded during a build, and every change to a dependency shows in a diff.
- Bad, because the repository grows by the size of every dependency, and the diffs of updates are too large to review.
- Bad, because binaries for four platforms would make every clone large.

### A commercial platform for supply chain security

- Good, because some platforms flag a malicious package by its behavior, before an advisory exists.
- Bad, because a template must not require a paid account, and the platform becomes one more party with access to the code.

## More Information

- [pnpm settings](https://pnpm.io/settings), [uv settings](https://docs.astral.sh/uv/reference/settings/), and [mise.lock](https://mise.jdx.dev/dev-tools/mise-lock.html)
- [OpenSSF Scorecard](https://github.com/ossf/scorecard)
- The pins of tools are in TPL-0022, the rule of one tool per concern in TPL-0023, and the updates in TPL-0025.
