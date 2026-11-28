# Configure the repository on GitHub

Some rules of this repository live in the settings of the repository on GitHub, not in its files.
CI protects `main` only if a ruleset requires the check of CI, and Renovate merges an update only if the repository allows automatic merges.
The script `tools/github/apply-settings.sh` writes these settings, so that they can be reviewed in a pull request and applied again at any time.
This guide explains how to run the script, what each setting does, and the steps that the script cannot do: the release App, the environments of the deployment, and the Renovate app.

## Apply the settings

You need `gh`, logged in as an administrator of the repository.
Run the script through `make`, which puts the pinned `gh` and `jq` on the `PATH`:

```sh
make github-settings                        # dry run: print what differs, change nothing
make github-settings APPLY=1                # change what differs
make github-settings REPO=<owner>/<name>    # another repository than the current one
```

Each line of the output names one setting:

- `ok` means that the setting already has the desired value.
- `change` shows the current and the desired value; with `APPLY=1`, the script writes the desired value.
- `note` reports something that the script cannot change.

The script compares every setting before it writes, so a second run with `APPLY=1` finds nothing to change.

A team that copies the template changes two values at the top of the script before the first run: `is_template=false`, and its own `topics`.

## What the script sets

### Repository

| Setting | Value | Why |
|---|---|---|
| Template repository | on | People create their projects from this repository. |
| Topics | `go`, `nextjs`, `ddd`, and others | People find the template by them. |
| Merge methods | squash only | Every pull request becomes one commit on `main`, and the history stays linear. |
| Squash commit message | the title of the pull request, then the messages of its commits | release-please reads the title on `main`, so CI checks that the title is a Conventional Commit (`make lint-pr-title`). |
| Delete head branches | on | A merged branch has no further use. |
| Automatic merges | allowed | Renovate merges patch, digest, and pin updates once the required check passes. |

### Ruleset of the default branch

The ruleset `main` applies to the default branch and has no bypass, not even for administrators:

- Every change arrives through a pull request. No approving review is required, because a single maintainer cannot approve their own pull request; a team raises the count.
- The check `check` must pass, and it must come from GitHub Actions. `check` is the last job of `.github/workflows/ci.yml` and fails when any other job of CI fails, so adding a job to CI never changes the ruleset.
- The history is linear, and pull requests merge only by squashing.
- Nobody can force-push to the branch or delete it.

The ruleset does not require a branch to be up to date with `main` before it merges. That would make every open pull request, including the ones from Renovate, wait for a rebase after each merge.

### Security

| Setting | Value | Why |
|---|---|---|
| Secret scanning and push protection | on | GitHub refuses a push that contains a known kind of secret. `make check` scans for secrets too (gitleaks), before the commit. |
| Private vulnerability reporting | on | `SECURITY.md` asks people to report through it. GitHub offers it for public repositories only. |
| Dependabot alerts | on | GitHub reports known vulnerabilities in the dependencies that its dependency graph sees. |
| Dependabot security updates | off | Renovate proposes the fixes, with the same commit messages and checks as every other update. |
| Code scanning default setup | off | `.github/workflows/codeql.yml` runs CodeQL instead, and the two cannot run together. |

### GitHub Actions

| Setting | Value | Why |
|---|---|---|
| Actions pinned to a full commit SHA | required | GitHub refuses a workflow that uses an action by a tag or a branch. `make check` checks the same (pinact, ghalint). |
| Default permissions of `GITHUB_TOKEN` | read | Each job asks for the permissions that it needs. |
| GitHub Actions may create and approve pull requests | no | A workflow cannot approve its own changes. Releases use an App instead (below). |

### Environment `release`

The environment `release` holds the private key of the release App, and only jobs that run on the default branch may use it.
A workflow on another branch therefore cannot read the key, even if someone edits the workflow.

## Set up the release App

`.github/workflows/release.yml` runs release-please on every push to `main`.
release-please keeps a release pull request open, titled `chore(main): release <version>`, which updates `CHANGELOG.md` and the version in `.github/release-please-manifest.json`.
Merging that pull request tags the release (`v0.1.0`) and publishes its release notes.

release-please writes with the token of a GitHub App, because the settings above forbid GitHub Actions to create pull requests, and because CI does not run on a pull request opened with `GITHUB_TOKEN`.
Until the App exists, the release workflow is skipped. To create it:

1. Create a GitHub App under the account that owns the repository (**Settings** > **Developer settings** > **GitHub Apps** > **New GitHub App**). It needs no webhook and no callback URL.
2. Give it these repository permissions, and no others: **Contents**: read and write, **Pull requests**: read and write, **Issues**: read and write.
3. Install the App on this repository only.
4. Generate a private key for the App.
5. Store the client ID as a variable of the repository, and the private key as a secret of the environment `release`:

   ```sh
   make github-settings APPLY=1   # creates the environment release first
   gh variable set RELEASE_APP_CLIENT_ID --body '<client ID>'
   gh secret set RELEASE_APP_PRIVATE_KEY --env release < private-key.pem
   ```

6. Delete the downloaded key file.

The next push to `main` opens the first release pull request, for version 0.1.0.
From then on, the commit messages on `main` decide the next version:

- `feat` raises the minor version; `fix` and every other type raise the patch version.
- A breaking change (`feat!:`, or a `BREAKING CHANGE:` footer) raises the minor version while the version is below 1.0.0, and the major version after that.
- `feat`, `fix`, `perf`, `revert`, `build`, `ci`, and `docs` appear in the release notes. `refactor`, `test`, `style`, and `chore` do not, and a release that would contain only those does not happen.

## Set up the environments of the deployment

The workflows `Plan` and `Deploy` reach the AWS accounts of stg and prd (TPL-0018). They need two GitHub environments and five repository variables, which the script neither sets nor compares. Set them by hand once the accounts are prepared, as [Deploy to AWS](deploy.md#prepare-an-account) describes; until the variable of a role is set, the jobs that need it are skipped.

| Setting | Value | Why |
|---|---|---|
| Environment `stg` | Deployment branches: `main` only | The role `github-deploy` of stg trusts a job of this environment alone, so only a merge into `main` deploys. |
| Environment `prd` | Deployment branches: `main` only. Required reviewers: at least one person | The job of prd waits until that person approves it. |
| Variable `AWS_REGION` | The Region of the environments, such as `ap-northeast-1` | Every job signs in to AWS there. |
| Variables `AWS_PLAN_ROLE_ARN_STG` and `AWS_PLAN_ROLE_ARN_PRD` | The ARN of the role `github-plan` of each account, which `make infra-bootstrap` prints | The jobs of `Plan` assume it. |
| Variables `AWS_DEPLOY_ROLE_ARN_STG` and `AWS_DEPLOY_ROLE_ARN_PRD` | The ARN of the role `github-deploy` of each account | The jobs of `Deploy` assume it. |

Create the environments in **Settings** > **Environments**, and set the variables with `gh`:

```sh
gh variable set AWS_REGION --body ap-northeast-1
gh variable set AWS_PLAN_ROLE_ARN_STG --body '<ARN of github-plan in stg>'
gh variable set AWS_DEPLOY_ROLE_ARN_STG --body '<ARN of github-deploy in stg>'
```

The variables are not secrets: an ARN grants nothing, and the trust policy of each role decides which jobs may assume it.

## Install Renovate

Install the [Renovate GitHub App](https://github.com/apps/renovate) on the repository.
Renovate reads `renovate.json` and opens one pull request per update, or per group of updates, every Monday:

- It waits three days after a release, as pnpm and uv do.
- Patch, digest, and pin updates merge by themselves once `check` passes. People review minor updates.
- A major update waits on the Dependency Dashboard. Tick its checkbox there to get its pull request.
- Security fixes arrive at once, outside the schedule.
- The lockfiles are refreshed on the first day of each month.
- The Dependency Dashboard issue lists every pending update.

Renovate updates `mise.lock` together with `mise.toml`.
If a pull request from Renovate changes `mise.toml` but not `mise.lock`, `make setup` fails in CI.
Check out the branch of that pull request, run `make tools-lock`, and push the result.

Renovate also updates the Terraform providers of `infra/live/providers.hcl`, with those of `infra/local/`, in one pull request (the group `Terraform providers`).
It does not write `infra/live/providers.lock.hcl`, so every Terragrunt run of that branch fails, `validate-live` in CI among them.
Check out the branch, run `make infra-lock`, and push the result.

When Renovate renames an option, the Dependency Dashboard offers a pull request that migrates `renovate.json`.
That pull request fails `lint-schema` while the schema that check-jsonschema bundles does not know the new form.
Renovate reads the old form in the meantime, so close the pull request, and migrate in the pull request that updates check-jsonschema.

pnpm and uv refuse a release younger than three days, which includes a security fix released today.
To take such a fix at once, exclude the package from that rule in `pnpm-workspace.yaml` (`minimumReleaseAgeExclude`) or in `pyproject.toml` (`exclude-newer-package`), and remove the exception once the release is three days old.
