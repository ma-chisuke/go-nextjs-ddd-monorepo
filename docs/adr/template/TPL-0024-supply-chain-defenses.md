---
status: accepted
date: 2026-11-19
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

Across the lockfiles, Trivy looks for known vulnerabilities and for dependencies under forbidden licenses (`scan-deps` in `make check-all`, and TPL-0019). The license scan covers only what ships with the product, so it leaves out development dependencies and the `tools/` module. Trivy reports a vulnerability from the day a fixed version exists (`ignore-unfixed` in `trivy.yaml`): a finding without a possible fix would fail every pull request until the maintainers of the dependency release one. Until then, Dependabot alerts list the vulnerability, and govulncheck still fails when Go code that is called is affected. Trivy is the one scanner for misconfiguration, vulnerabilities, licenses, and SBOMs: one pinned binary covers all four, so there is no second scanner to pin and to watch. For Go, govulncheck narrows the vulnerabilities to the code that is actually called (TPL-0023).

For secrets, two tools look for a credential in the code.

- gitleaks scans the staged changes before a commit, and the working tree in `make check` (`lint-secrets`).
- GitHub secret scanning and push protection check every push.

For the repository, the template uses these services on GitHub.

- CodeQL analyzes the Go and TypeScript code (`.github/workflows/codeql.yml`).
- OpenSSF Scorecard rates the security practices of the repository (`.github/workflows/scorecard.yml`).
- Private vulnerability reporting lets people report a vulnerability without making it public.
- Dependabot raises alerts for vulnerable dependencies. It opens no pull requests: updates come from Renovate (TPL-0025).

For containers, these rules apply.

- Base images are minimal and pinned by digest.
- Containers run as a user other than root. In Docker Compose, the services of the product, the frontend included, also run with a read-only file system, without capabilities, and without new privileges (`x-context-service` in `compose.yaml`).
- Trivy scans every image and writes its SBOM.
- Builds get provenance through GitHub artifact attestations. This is not built yet: the workflow `Deploy` builds the images and pushes them to Amazon ECR without attesting them.
- The repositories of Amazon ECR keep tags immutable, so a tag names one image for good, and scan each image when it is pushed.
- The image that prepares the database (`db/Dockerfile`) builds on the image of PostgreSQL that `compose.yaml` runs, pinned by digest, and runs as the user 999. It adds the release of pgschema that `mise.toml` pins, which `ADD --checksum` checks against the SHA-256 that `mise.lock` records for Linux.
- The image of the frontend (`frontend/Dockerfile`) builds on the official Node image, pinned by digest. Node no longer ships Corepack, so the build installs pnpm from its package on the registry of npm, which `ADD --checksum` checks against its SHA-256. It installs the dependencies of the frontend alone from `pnpm-lock.yaml`, under the settings of `pnpm-workspace.yaml`, and the image holds the standalone server of Next.js on distroless Node, as the user 65532.

For the infrastructure in AWS, these rules apply.

- The providers of every Terragrunt unit are pinned in `infra/live/providers.hcl`, and locked with their hashes for four platforms in `infra/live/providers.lock.hcl`. Every unit initializes with `-lockfile=readonly`, so a provider that the lock file does not name fails to install (TPL-0015).
- Trivy checks the Terraform code for misconfiguration (`scan-config`). A finding that review accepted is listed in `.trivyignore.yaml`, with the files that it is accepted in and the reason; everywhere else the finding still fails. The file has one entry: AWS-0053, a load balancer that faces the internet, in `infra/modules/web/main.tf` alone, the one public entry of the system (TPL-0014), which the owner of the template approved. The instructions for agents forbid them to add an entry.
- The tasks reach the AWS services through VPC endpoints, and no security group allows egress to an address range outside the VPC (TPL-0014). Trivy rates such egress as critical (AWS-0104), and no entry accepts it.
- The load balancer keeps no access logs. A load balancer writes them only to a bucket encrypted with keys that Amazon S3 manages, and Trivy requires a customer managed key for a bucket (AWS-0132). The decision on the logs, and on that finding, is open.

The local environment runs software that the product does not ship, and pins it too (TPL-0017).

- Mailpit is pinned by digest in `compose.yaml`, at v1.33.1 rather than the newest release. That release was one day old when Mailpit was added, younger than the three days that the other updates wait.
- AWS Emulator is pinned three ways in `infra/local/aws-emulator/Dockerfile`: by the version of its package, by the SHA-256 of the package for each architecture, and by the signature of AWS, which `rpm` checks against the public key in `signing-key.asc`. Its base image is pinned by digest, and it runs as the user 65532.
- The AWS provider of Terraform is locked with its hashes for four platforms in `infra/local/.terraform.lock.hcl`.
- The browser of the end-to-end test is the build of Chromium that the version of `@playwright/test` in `pnpm-lock.yaml` names. `tools/scripts/stack.sh e2e` downloads it into `.cache/ms-playwright/` when it is missing, not `make setup`. This repository records no checksum of the browser itself; it trusts the release of Playwright that pnpm installed under the rules above.

The AWS SDK for Go v2 is a module in `backend/go.mod`, under the rules for Go modules above.

Two parts of this decision differ from the first plan, on purpose.

- The first plan avoided `trivy-action` only. The same attack rewrote the tags of `setup-trivy`, so neither action is used.
- The first plan named Trivy or syft for SBOMs. Trivy writes them, because it already scans the images, and each concern has one tool (TPL-0023).

### Consequences

- Good, because a moved tag, a replaced release, or a tampered download fails at install time instead of running.
- Good, because a malicious npm or Python release that is found and pulled within three days never reaches this repository.
- Good, because a compromised step in CI holds only the permissions that its job asked for, and only for as long as the job runs, which its timeout limits.
- Bad, because updates, security fixes included, arrive three days late unless someone overrides the wait. A rebuild of a base image under the same tag is the exception: Renovate proposes its digest at once (TPL-0025), so a rebuild that someone tampered with is not held back for three days either.
- Bad, because adding a tool has friction: an entry in `mise.toml`, a lock for four platforms, and sometimes an entry in `allowBuilds`.
- Bad, because some tools publish no provenance. For them, `mise.lock` records only a checksum (gitleaks, shellcheck, and Vale among them). A checksum shows that every machine gets the file that was locked, not that the file is what its author built.
- Bad, because the repository sets no wait for Go modules or for the tools in `mise.toml`. A version that a person adds by hand, with `go get` or in `mise.toml`, arrives at once, and only the updates that Renovate proposes wait (TPL-0025).
- Bad, because several defenses on GitHub are settings, not files. `make github-settings` compares them with `docs/guides/github-settings.md`, but `make check` cannot see them.
- Bad, because no Renovate rule updates the version of AWS Emulator yet. A person changes the version and the two checksums in its Dockerfile by hand, and no wait of three days applies (TPL-0025).
- Bad, because no Renovate rule updates the version of pnpm and the checksum of its package in `frontend/Dockerfile` either. When an update moves pnpm in `mise.toml` and `package.json`, a person changes both in the Dockerfile by hand (TPL-0025).
- Bad, because no Renovate rule updates pgschema in `db/Dockerfile` either. When an update moves pgschema in `mise.toml`, a person changes its version and its checksums in the Dockerfile, and `make test-db-image` fails until then (TPL-0025).
- Bad, because the images in Amazon ECR carry no provenance yet, and `scan-images` and `make sbom` cover the images of Docker Compose only, not the image that prepares the database.
- Bad, because the public load balancer keeps no access logs until a decision on AWS-0132.

### Confirmation

- `make check` runs `lint-actions-pins`, `lint-actions-security`, and `lint-actions-policy` on the workflows, `lint-secrets` on the files, and `scan-config` with the checks built into Trivy.
- `make setup` runs `mise install` under `locked = true`, `pnpm install --frozen-lockfile` with the settings in `pnpm-workspace.yaml`, and `uv sync --locked`. CI runs the same target.
- `make check-all` runs `scan-deps` and `vuln-go`.
- Review confirms that no workflow uses `trivy-action` or `setup-trivy`. No check enforces it.
- CodeQL and Scorecard run as workflows. Secret scanning, push protection, private vulnerability reporting, and Dependabot alerts are settings of the repository: `make github-settings` turns them on, and reports any that differ from `docs/guides/github-settings.md`.
- For containers, `make check` runs `lint-image-pins`, which fails on a base image without a digest, and `scan-config`, whose Trivy checks fail a Dockerfile whose image runs as root. Both check the Dockerfiles of AWS Emulator, of the frontend, and of the image that prepares the database too. The images of the contexts (`backend/Dockerfile`) run as the user `nonroot` of distroless static, and the image of the frontend (`frontend/Dockerfile`) as the user 65532 of distroless Node.
- The build of the image of the frontend fails when the package of pnpm does not match its checksum.
- The build of the image of AWS Emulator fails when the package does not match its checksum or its signature.
- `make check-all` runs `scan-images`, which scans the image of every context and the image of the frontend with Trivy for known vulnerabilities. `make sbom` writes a CycloneDX SBOM of each of those images to `dist/sbom/`. Neither covers the image of AWS Emulator, a tool of the local environment that nothing ships.
- `scan-config` applies `.trivyignore.yaml`, which accepts a finding only in the files that its entry names, so a load balancer that faces the internet in any other file fails `make check`.
- `make test-db-image`, in `make check-all`, fails when the pgschema in the image that prepares the database differs from the one that `mise.toml` pins. The build of that image fails when the binary of pgschema does not match its checksum.
- Every Terragrunt run fails on a provider that `infra/live/providers.lock.hcl` does not name.
- Review confirms that no entry of `.trivyignore.yaml` was added without the approval of a person; no check can.
- Provenance through GitHub artifact attestations is not built. The workflow `Deploy` publishes the images, and attesting them, with the SBOMs of `make sbom`, is left to a later change.

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
