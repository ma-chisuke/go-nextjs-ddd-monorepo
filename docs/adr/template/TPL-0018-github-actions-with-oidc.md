---
status: accepted
date: 2026-11-19
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

- Apart from the steps that check out the code, install mise, restore the caches, and sign in to AWS, each step in a workflow calls a `make` target, such as `make setup` or `make check-all`, and holds no logic of its own. CI, people, and agents therefore run the same commands.
- A job signs in to AWS with the action `aws-actions/configure-aws-credentials`, pinned to the commit of v6.3.0. GitHub gives the token with which a job requests its OIDC token to actions, and not to `run` steps, so the sign-in cannot be a `make` target.
- The required status check has one fixed name, `check`. It is the last job of the workflow and depends on every other job, so the ruleset does not change when a job is added.
- The job `check` runs even when a job it depends on fails or is skipped, and it fails if any of those jobs neither succeeded nor was skipped. Otherwise a single failed job would make GitHub skip `check`, and GitHub counts a skipped job as a passing check.
- A new run for a branch cancels the runs still in progress for the same branch.
- Workflows cache tools and dependencies, so that a run does not download them again.
- The job that runs `make check` checks every file. The job that adds the slow checks of `make check-all` narrows them to the changed files on a pull request (`CHECK_SCOPE=changed`), and checks every file on `main` and whenever a change touches a tool pin or the definition of a check (TPL-0023).
- Every action is pinned to a full commit SHA.
- `permissions` are read-only by default, and each job asks for the permissions it needs.
- Every job sets `timeout-minutes`.

The rules for deployments, which the infrastructure milestone built, are as follows.

- A deployment job assumes an IAM role through GitHub's OIDC provider and receives short-lived credentials. No AWS access key is stored on GitHub.
- The trust policy of each role that applies changes names this repository and the GitHub Environment of the deployment, so no other repository or environment can assume the role.
- Each account also has a read-only role for `terragrunt plan` on pull requests (TPL-0015). Its trust policy names this repository and its pull requests, not an Environment, and the role can read the infrastructure but not change it.
- The stg and prd environments each have their own roles, in their own AWS account: `github-plan` and `github-deploy`, from the module `infra/modules/github-oidc` and the unit `ci` (TPL-0015).
- Both trust policies compare the whole subject of the token with `StringEquals`. `github-plan` trusts `repo:<repository>:pull_request`, and `github-deploy` trusts `repo:<repository>:environment:<environment>`. The repository is one, without a wildcard.
- GitHub writes the repository in the subject in an immutable form for repositories created after July 15, 2026: `repo:<owner>@<owner id>/<name>@<repository id>:...`. This repository is one of them, and `infra/live/system.hcl` holds the repository in that form.
- `github-plan` has `ReadOnlyAccess`, and may write and delete the lock files of the state. `github-deploy` has `AdministratorAccess` by default: it creates IAM roles, so a narrower policy would not limit what it can reach, and what limits it is who may assume it.
- The workflow `Plan` (`.github/workflows/plan.yml`) runs `make infra-plan` for stg and for prd on a pull request, as `github-plan`. It skips a pull request from a fork, which gets no token.
- The workflow `Deploy` (`.github/workflows/deploy.yml`) runs `make deploy` after a push to `main`: stg first, then prd in the GitHub Environment `prd`, which requires the approval of a reviewer. One deployment runs at a time, in the order of the merges, and none is cancelled half-way.
- A job is skipped until the repository variable with the ARN of its role is set: `AWS_PLAN_ROLE_ARN_STG`, `AWS_PLAN_ROLE_ARN_PRD`, `AWS_DEPLOY_ROLE_ARN_STG`, or `AWS_DEPLOY_ROLE_ARN_PRD`. Every job also reads the Region from `AWS_REGION`. These variables are not secrets.
- `make deploy ENV=prd` refuses to run outside GitHub Actions, so prd changes only through the job that a person approved.

This differs from the first plan in one point: the first plan allowed no step before `make` but the checkout, mise, and the caches. Signing in to AWS is a fourth kind of step, because GitHub gives the request token for OIDC to actions alone.

### Consequences

- Good, because a failure in CI can be reproduced on a laptop with the same `make` command.
- Good, because a leaked log or a compromised step can expose only short-lived credentials, and only the named repository and environment can obtain credentials that change anything.
- Good, because the ruleset names a single check, so jobs can be added, split, or renamed without a change on GitHub.
- Good, because the checks, the approval for prd, and the logs sit next to the pull request.
- Bad, because the condition of the job `check` must be written with care: a mistake in it lets a failed job through.
- Bad, because the trust policy must be exact. A condition that matches too much, such as any repository of the owner, gives the role to code that nobody reviewed.
- Bad, because an outage of GitHub Actions stops both CI and deployments.
- Bad, because on a pull request the slow checks look only at the changed files, so a problem in a file that the pull request did not touch, such as a dead link in another document, shows up only on `main`.
- Bad, because the pinned SHAs must be kept up to date. Renovate does that (TPL-0025).
- Bad, because the role that deploys can do anything in its account. Its trust policy, and the reviewer of the Environment for prd, are what keep it out of the hands of unreviewed code.
- Bad, because the Environments, their reviewers, and the repository variables are settings on GitHub, outside the files and outside `make github-settings`, so a person sets them up for each copy of the template ([GitHub settings](../../guides/github-settings.md)).

### Confirmation

- `make check` runs four checks on the workflows in `.github/workflows/` (`make/actions.mk`): actionlint for syntax and expressions, zizmor for security weaknesses such as template injection and broad permissions, ghalint for the policy on permissions and timeouts, and pinact for pins to full commit SHAs.
- gitleaks, which `make check` runs, and GitHub secret scanning find an AWS key that is written into a file. A key stored as a secret on GitHub is out of their reach, so review of the repository settings confirms that there is none.
- Review confirms that each step calls `make` and holds no logic of its own; no check enforces it. `make github-settings` compares the ruleset, which requires `check`, with `docs/guides/github-settings.md`.
- The tests of `infra/modules/github-oidc` (`validate-terraform`, in `make check-all`) check that only a job of a pull request of the repository may assume the read-only role, and only a job of the GitHub Environment of its environment the role that deploys. They also check that the immutable form of a repository is accepted, and that a wildcard in the repository, or a subject smuggled into it, is rejected.
- `infra.sh deploy` refuses prd unless `GITHUB_ACTIONS` is `true`.
- Not yet verified, because nothing has been applied to AWS: the subjects of a pull request and of an Environment in the immutable form, which the documentation of GitHub shows for a branch only, and whether `ReadOnlyAccess` is enough for every refresh in a plan, such as the versions of the secrets ([Deploy to AWS](../../guides/deploy.md#what-the-first-apply-must-confirm)).

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
