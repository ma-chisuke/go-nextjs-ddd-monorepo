---
status: accepted
date: 2026-11-19
---

# TPL-0025: Update dependencies and tools with Renovate

## Context and Problem Statement

Everything in this repository is pinned: the versions of tools in `mise.toml`, packages in lockfiles, and actions by commit SHA. Later, container images are pinned by digest, and Terraform providers by version (TPL-0022, TPL-0024).
Pins make every machine run the same code, but they do not move on their own.
A pin that nobody updates keeps a known vulnerability, and an update that waits for months turns into a migration.

The pins live in many formats: Go modules, npm packages, Python packages, Docker images, actions, the tools in `mise.toml`, and Terraform providers.
Updating them by hand is work that nobody does on time, and an updater that understands only some of these formats leaves the others behind.

How do we keep every pin up to date without making updates a burden or a risk?

## Decision Drivers

- One updater covers every kind of pin, `mise.toml` included.
- An update arrives as a pull request, and the checks verify it like any other change.
- Updates wait three days after a release, as pnpm and uv do (TPL-0024).
- Routine updates need no person, and risky ones do.
- The number of pull requests stays small enough to review.

## Considered Options

- Renovate for every update, with Dependabot for alerts only
- Dependabot version updates
- Manual updates
- Both Renovate and Dependabot version updates

## Decision Outcome

Chosen option: "Renovate for every update, with Dependabot for alerts only", because Renovate covers every kind of pin in this repository, `mise.toml` included, and Dependabot's version updates do not.

The rules are as follows.

- Renovate updates Go modules, npm packages, Python packages, the digests of Docker images, the SHAs of actions, the tools in `mise.toml`, and Terraform providers.
- The providers of the environments are pinned in `infra/live/providers.hcl`, which the Terraform manager of Renovate reads (`terraform` in `renovate.json`). The group `Terraform providers` moves them together with those of `infra/local/`, which use the same providers. Renovate does not write `infra/live/providers.lock.hcl`, so a person runs `make infra-lock` on the branch of the update, as `make tools-lock` for `mise.lock`; until then, every Terragrunt run of the branch fails.
- It waits three days after a release before it proposes the release.
- It groups related updates into one pull request.
- It opens pull requests on a weekly schedule. Security fixes do not wait for the schedule.
- A new digest of a base image under the same tag waits for neither the schedule nor the three days. It is a rebuild of the image with fixed packages, and `scan-images` fails every change to a source of an image from the day a package has a fix until the pin moves: in November 2026 that stopped CI twice within a week, and the second time the rebuilt image had been out for three days. The update merges only when the checks pass, the scan, the smoke test, and the scenarios in a browser among them. The wait of three days is given up for these images alone, which come from the publishers that TPL-0024 names.
- It merges patch, digest, and pin updates on its own when the required check passes. A minor update waits for a person. A major update gets no pull request until a person ticks its checkbox on the Dependency Dashboard: it is a decision, such as the next PostgreSQL, and a pull request that stays open for months hides the ones that need a look.
- An update changes a pin or a lockfile, so the checks of its pull request cover every file, not only the changed ones (TPL-0023). A new version of a tool runs on the whole repository before the update merges.
- Dependabot stays on for alerts only, and opens no pull requests.
- The configuration is `renovate.json`.

### Consequences

- Good, because every pin is updated in the same way, through a pull request that the checks verify.
- Good, because patch-level updates need no person, so people review only the updates that are more likely to break something.
- Good, because Renovate waits as long as pnpm and uv do, so it does not propose a release that they would refuse to install.
- Bad, because the pull requests keep coming: even grouped and weekly, they take review time every week.
- Bad, because an automatic merge is only as safe as the checks. A patch release that breaks behavior that the tests do not cover is merged without a person.
- Bad, because the Renovate app needs write access to the repository to push branches and open pull requests, which makes it one more party to trust.
- Bad, because a mistake in the configuration can quietly stop the updates for one kind of pin, and the schema cannot tell.
- Bad, because a pin that no manager of Renovate reads is not updated. The version of AWS Emulator and the checksums of its package, in build arguments of `infra/local/aws-emulator/Dockerfile`, are such a pin: a person updates them by hand (TPL-0017). The version of pnpm and the checksum of its package, in the `ADD` of `frontend/Dockerfile`, are another: the group `pnpm` of `renovate.json` moves `mise.toml`, `package.json`, and `pnpm-lock.yaml`, and a person changes the Dockerfile in the same pull request (TPL-0024). The version of pgschema and the checksums of its binaries, in `db/Dockerfile`, are a third: when Renovate moves pgschema in `mise.toml`, a person changes the Dockerfile, and `make test-db-image` fails until the two agree.
- Bad, because the schema that check-jsonschema bundles is older than the Renovate that the app runs. When Renovate renames an option, the pull request in which it migrates the configuration fails `lint-schema` until a release of check-jsonschema bundles the new schema. Open since November 28, 2026: `dependencyDashboard` is `true`, and Renovate now writes it as an object.
- Bad, because an update of the providers needs a person to run `make infra-lock`, so it never merges on its own.

### Confirmation

- `make check` validates `renovate.json` against the Renovate schema that check-jsonschema bundles (target `lint-schema`), without the network.
- The schema catches a malformed configuration, not a wrong one. Review of the pull requests that Renovate opens confirms that it covers every kind of pin.
- Dependabot alerts and automatic merges are settings of the repository, which `make github-settings` turns on (`docs/guides/github-settings.md`). The Renovate app is installed by hand, as the guide describes.

## Pros and Cons of the Options

### Renovate for every update, with Dependabot for alerts only

- Good, because one tool, one configuration, and one style of pull request cover every pin.
- Good, because its configuration controls the wait, the groups, the schedule, and the automatic merges.
- Bad, because it is a third-party app with write access to the repository.

### Dependabot version updates

- Good, because it is built into GitHub, with no app to install.
- Bad, because it does not update `mise.toml`, so the pins of tools would need a second process.
- Bad, because it does not merge on its own; that needs a separate workflow.

### Manual updates

- Good, because nothing changes without a person.
- Bad, because updates happen only when someone remembers them, so pins fall behind, and a security fix waits until someone notices it.

### Both Renovate and Dependabot version updates

- Good, because if one updater stops working, the other still proposes updates.
- Bad, because both open pull requests for the same packages, which doubles the noise and makes the two compete.

## More Information

- [Renovate documentation](https://docs.renovatebot.com/)
- The pins are described in TPL-0022, and the wait of three days in TPL-0024.
