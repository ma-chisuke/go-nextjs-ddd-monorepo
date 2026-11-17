---
status: accepted
date: 2026-11-17
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
- It waits three days after a release before it proposes the release.
- It groups related updates into one pull request.
- It opens pull requests on a weekly schedule. Security fixes do not wait for the schedule.
- It merges a patch-level update on its own when the checks pass. Minor and major updates wait for a person.
- Dependabot stays on for alerts only, and opens no pull requests.
- The configuration is `renovate.json5`.

### Consequences

- Good, because every pin is updated in the same way, through a pull request that the checks verify.
- Good, because patch-level updates need no person, so people review only the updates that are more likely to break something.
- Good, because Renovate waits as long as pnpm and uv do, so it does not propose a release that they would refuse to install.
- Bad, because the pull requests keep coming: even grouped and weekly, they take review time every week.
- Bad, because an automatic merge is only as safe as the checks. A patch release that breaks behavior that the tests do not cover is merged without a person.
- Bad, because the Renovate app needs write access to the repository to push branches and open pull requests, which makes it one more party to trust.
- Bad, because a mistake in the configuration can quietly stop the updates for one kind of pin, and the schema cannot tell.

### Confirmation

- `make check` validates `renovate.json5` against the Renovate schema that check-jsonschema bundles (target `lint-schema`), without the network.
- The schema catches a malformed configuration, not a wrong one. Review of the pull requests that Renovate opens confirms that it covers every kind of pin.
- The Renovate app and Dependabot alerts are settings on GitHub. They are turned on when the repository is set up on GitHub, and review confirms them there.

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
