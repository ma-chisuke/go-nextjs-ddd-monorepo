---
status: accepted
date: 2026-11-17
---

# TPL-0018: Run CI/CD on GitHub Actions, and reach AWS through OIDC

## Context and Problem Statement

The code, the pull requests, and the reviews of this template live on GitHub.
Every pull request needs the checks that people and coding agents run on their own machines.
Later, deployments to AWS need a place to run as well, and stg and prd live in separate AWS accounts (TPL-0015).

Any choice of CI brings two problems.
First, CI tends to grow commands of its own. A check then passes on a laptop and fails in CI, or the reverse, and nobody can reproduce the failure.
Second, a deployment needs credentials for AWS. A long-lived access key stored in CI works from anywhere, for anyone who obtains it, until someone rotates it. It can leak through a log, through a compromised action, or through a mistake in a workflow.

Where does CI run, how do its workflows stay the same as the local checks, and how does a deployment reach AWS without a stored key?

## Decision Drivers

- CI, people, and agents run the same commands and get the same results.
- No long-lived credential for AWS exists anywhere.
- A change to prd needs the approval of a person.
- Adding or renaming a job must not require a change to the settings of the repository on GitHub.
- A workflow runs with access to the repository and its tokens, so it gets the same scrutiny as other code.
- A team that copies the template needs no service beyond GitHub and AWS.

## Considered Options

- GitHub Actions, with access to AWS through GitHub's OIDC provider
- GitHub Actions, with AWS access keys stored as secrets
- AWS CodePipeline and AWS CodeBuild
- Another hosted CI service

## Decision Outcome

Chosen option: "GitHub Actions, with access to AWS through GitHub's OIDC provider", because it runs next to the pull requests and leaves no key to steal.

The rules for workflows are as follows.

- A step in a workflow calls a `make` target, such as `make setup` or `make check-all`, and holds no logic of its own. CI, people, and agents therefore run the same commands.
- The required status check has one fixed name, `check`. It is the last job of the workflow and depends on every other job, so the ruleset does not change when a job is added.
- The job `check` runs even when a job it depends on fails or is skipped, and it fails if any of those jobs neither succeeded nor was skipped. Otherwise a single failed job would make GitHub skip `check`, and GitHub counts a skipped job as a passing check.
- A new run for a branch cancels the runs still in progress for the same branch.
- Workflows cache tools and dependencies, so that a run does not download them again.
- A slow job runs only when the files it covers change.
- Every action is pinned to a full commit SHA.
- `permissions` are read-only by default, and each job asks for the permissions it needs.
- Every job sets `timeout-minutes`.

The rules for deployments, which arrive with the infrastructure milestone, are as follows.

- A deployment job assumes an IAM role through GitHub's OIDC provider and receives short-lived credentials. No AWS access key is stored on GitHub.
- The trust policy of each role names this repository and the GitHub Environment of the deployment, so no other repository or environment can assume the role.
- The stg and prd environments each have their own role, in their own AWS account.
- A deployment to prd goes through a GitHub Environment that requires the approval of a reviewer.

### Consequences

- Good, because a failure in CI can be reproduced on a laptop with the same `make` command.
- Good, because a leaked log or a compromised step can expose only short-lived credentials, and only the named repository and environment can obtain them.
- Good, because the ruleset names a single check, so jobs can be added, split, or renamed without a change on GitHub.
- Good, because the checks, the approval for prd, and the logs sit next to the pull request.
- Bad, because the condition of the job `check` must be written with care: a mistake in it lets a failed job through.
- Bad, because the trust policy must be exact. A condition that matches too much, such as any repository of the owner, gives the role to code that nobody reviewed.
- Bad, because an outage of GitHub Actions stops both CI and deployments.
- Bad, because a path filter that misses a file lets a change skip the job that should check it.
- Bad, because the pinned SHAs must be kept up to date. Renovate does that (TPL-0025).

### Confirmation

- `make check` runs four checks on the workflows in `.github/workflows/` (`make/actions.mk`): actionlint for syntax and expressions, zizmor for security weaknesses such as template injection and broad permissions, ghalint for the policy on permissions and timeouts, and pinact for pins to full commit SHAs.
- gitleaks, which `make check` runs, and GitHub secret scanning find an AWS key that is written into a file. A key stored as a secret on GitHub is out of their reach, so review of the repository settings confirms that there is none.
- Review confirms that each step calls `make` and holds no logic of its own, and that the ruleset requires `check`. No check enforces these.
- The IAM roles and their trust policies arrive with the infrastructure milestone. Until then, no workflow needs access to AWS, and the rules for deployments are not yet in effect.

## Pros and Cons of the Options

### GitHub Actions, with access to AWS through GitHub's OIDC provider

- Good, because no long-lived credential exists, so there is nothing to rotate and nothing to leak for long.
- Good, because the trust policy decides which repository and environment may deploy, and AWS enforces it.
- Good, because GitHub Environments give prd an approval step without another tool.
- Good, because Actions comes with every repository on GitHub, and its hosted runners are free for public repositories.
- Bad, because third-party actions are code from other people that runs with the permissions of the job, and pinning and checking them is work (TPL-0024).
- Bad, because a trust policy is easy to get subtly wrong.

### GitHub Actions, with AWS access keys stored as secrets

- Good, because it is the quickest to set up.
- Bad, because the key is long-lived and works from anywhere: whoever reads it, from a log or through a compromised action, can use it until someone rotates it.
- Bad, because rotation is manual work that is easy to forget.

### AWS CodePipeline and AWS CodeBuild

- Good, because builds run inside AWS with IAM roles, so no key leaves AWS.
- Bad, because the checks of every pull request, including those from forks, would run in the owner's AWS account and need a connection back to GitHub to report their status.
- Bad, because a team that copies the template would need an AWS account before its first check could run.

### Another hosted CI service

- Good, because some services offer faster machines or richer caching.
- Bad, because it adds an account, a second place for permissions and secrets, and a connection to GitHub that must be kept secure.
- Bad, because the approval for prd would move out of GitHub into the controls of that service.

## More Information

- The defenses for actions and their pins are in TPL-0024, and the updates of the pins in TPL-0025.
- [actionlint](https://github.com/rhysd/actionlint), [zizmor](https://github.com/zizmorcore/zizmor), [ghalint](https://github.com/suzuki-shunsuke/ghalint), and [pinact](https://github.com/suzuki-shunsuke/pinact)
- [Configuring OpenID Connect in Amazon Web Services](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-in-aws)
