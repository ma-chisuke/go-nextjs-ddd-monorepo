---
status: accepted
date: 2026-11-17
---

# TPL-0023: Use one tool per concern, run every tool through make, and give `make check` a time budget

## Context and Problem Statement

A repository with Go, TypeScript, Terraform, shell scripts, YAML, and Markdown in two languages needs many formatters, linters, and scanners.
Many of these tools overlap. Two linters report the same finding, two formatters rewrite the same file in different ways, and two scanners report the same vulnerability in different words.
An agent that gets two findings for one problem, or two findings that disagree, wastes turns, or goes back and forth between them.

Checks also get slower as they accumulate.
People and agents run a check that takes minutes less often, and a slow check at the end of every agent turn makes every turn slow.

How do we choose tools so that each problem is reported once, and keep the checks fast enough to run on every change?

## Decision Drivers

- Inside `make`, each kind of finding comes from one tool.
- People, agents, CI, and the Git hooks run the same commands.
- `make check` is fast enough to run at the end of every agent turn.
- The output is short: one line per check, and the findings of failed checks only.
- A finding is one line that gives at least the file and the line, so that an agent can act on it without a person.
- The files that no check may touch are listed in one place.

## Considered Options

- One tool per concern, behind `make`, with a time budget for `make check`
- An aggregator such as MegaLinter or super-linter
- The pre-commit framework as the runner
- Every tool with its defaults, run only in CI

## Decision Outcome

Chosen option: "One tool per concern, behind `make`, with a time budget for `make check`", because it is the only option that controls both which tool reports what and how long a check takes.

Inside `make`, each concern has one tool. Targets marked "check-all" run only in `make check-all`. Targets marked "CI" are part of neither `make check` nor `make check-all`, and CI runs them in steps of their own.

| Concern | Tool | Target |
|---|---|---|
| Formatting of Go code | golangci-lint, with gofmt and goimports as formatters | `fmt-check-go` |
| Linting of Go code, including `go vet` | golangci-lint | `lint-go` |
| Go tests | `go test -race -shuffle=on` | `test-go` |
| The local system end to end: one scenario across the services in Docker Compose, over HTTP, with every response checked against its API document | `go test -tags smoke`, with kin-openapi (`backend/test/smoke`) | `smoke` (check-all) |
| Test coverage, reported without an external service | `go test -coverprofile` and octocov | `coverage` and `coverage-report` (CI) |
| Known vulnerabilities in Go code that is actually called | govulncheck | `vuln-go` (check-all) |
| Linting of the frontend, including the boundaries between features | ESLint, with eslint-config-next, typescript-eslint, and eslint-plugin-boundaries | `lint-frontend` |
| Types of the frontend | the TypeScript compiler (`tsc --noEmit`) | `lint-frontend-types` |
| Files, exports, and dependencies of the frontend that nothing uses | knip | `lint-frontend-unused` |
| Formatting of YAML, JSON, JSON5, JavaScript, TypeScript, and CSS | Prettier | `fmt-check-prettier` |
| Linting of YAML, such as duplicate keys and ambiguous values | yamllint | `lint-yaml` |
| Formatting and linting of TOML | taplo | `fmt-check-toml`, `lint-toml` |
| Configuration files against their schemas | check-jsonschema | `lint-schema`, `lint-schema-remote` (check-all) |
| Formatting of shell scripts | shfmt | `fmt-check-shell` |
| Linting of shell scripts | shellcheck | `lint-shell` |
| Linting of Dockerfiles | hadolint | `lint-dockerfile` |
| Pins of base images to digests (TPL-0024) | a pattern in `tools/scripts/image-pins.sh` | `lint-image-pins` |
| Syntax of workflows | actionlint | `lint-actions` |
| Security of workflows | zizmor | `lint-actions-security` |
| Policy of workflows: permissions and timeouts | ghalint | `lint-actions-policy` |
| Pins of actions to full commit SHAs | pinact | `lint-actions-pins` |
| Formatting of Terraform files | `terraform fmt` | `fmt-check-terraform` |
| Formatting of the other HCL files, such as `terragrunt.hcl` and `root.hcl` | `terragrunt hcl fmt` | `fmt-check-terragrunt` |
| Linting of Terraform code | tflint, with the AWS ruleset | `lint-terraform` |
| Module READMEs that are out of date | terraform-docs | `lint-terraform-docs` |
| Validity of Terraform code | `terraform validate` | `validate-terraform` (check-all) |
| SQL files | squawk | `lint-sql` |
| The DDL of each schema change, from a pgschema plan | squawk | `lint-sql-plan` (check-all) |
| Queries, prepared against the schema of their context | `sqlc vet` | `lint-sqlc` (check-all) |
| Comments on tables and columns, and the schema documents | tbls | `lint-db-docs` (check-all) |
| OpenAPI and AsyncAPI documents | Redocly CLI (`@redocly/cli`) | `lint-contracts` |
| Breaking changes to OpenAPI documents | oasdiff | `lint-contracts-breaking` |
| The JSON Schemas of events: their form, their examples, and the message of the AsyncAPI document that refers to each (TPL-0013) | the events command of this repository (`tools/cmd/events`) | `lint-events` |
| Breaking changes to the JSON Schemas of events | the events command of this repository | `lint-events-breaking` |
| Formatting and structure of Markdown | markdownlint-cli2 | `lint-md` |
| English prose | Vale, with the Google style | `lint-prose-en` |
| Japanese prose | textlint | `lint-prose-ja` |
| Links | lychee | `lint-links`, `lint-links-remote` (check-all) |
| Decision records and their index (TPL-0001) | the docs command of this repository (`tools/cmd/docs`) | `lint-adr` |
| Translations against their English files (TPL-0021) | the docs command of this repository | `lint-i18n` |
| The boundaries of the Go backend: imports between contexts and layers, a domain without I/O, routes registered by hand, schemas named in SQL, and the glossary of each context (TPL-0003, TPL-0004, TPL-0005, TPL-0007, TPL-0009) | the arch command of this repository (`tools/cmd/arch`) | `lint-arch` |
| Length of `AGENTS.md` (TPL-0020) | a line count in `make/agents.mk` | `lint-agents-md` |
| Spelling | typos | `lint-typos` |
| Line endings, final newlines, trailing spaces, and the style of indentation | editorconfig-checker | `lint-editorconfig` |
| Secrets | gitleaks | `lint-secrets` |
| Misconfiguration of Dockerfiles and Terraform | Trivy | `scan-config` |
| Vulnerabilities and licenses of dependencies | Trivy | `scan-deps` (check-all) |
| Vulnerabilities in the images of the services | Trivy | `scan-images` (check-all) |
| SBOMs of the images of the services | Trivy | `sbom`, which the workflow that publishes the images runs |
| The hooks for agents (TPL-0020) | sample events fed to each hook (`tools/tests/hooks_test.sh`) | `test-hooks` |
| The script that checks commit messages | sample subjects, each tried in several locales (`tools/tests/commit_msg_test.sh`) | `test-commit-msg` |
| The checks themselves: each fails on a known-bad input and passes on a known-good one | `tools/selftest/selftest.sh` | `selftest` (check-all) |
| Commit messages and pull request titles (Conventional Commits) | `tools/scripts/commit-msg.sh` | the `commit-msg` hook of lefthook, and `lint-pr-title` (CI) |

Schemathesis arrives with the milestone that adds it (TPL-0007).

These overlaps were resolved as follows.

- `go vet` runs inside golangci-lint, together with the other Go linters, and is not a separate step. gofmt and goimports run there too, as formatters.
- Spelling belongs to typos, so the spelling rule of Vale is off (`.vale.ini`).
- Vale reads English Markdown only, and textlint reads Japanese Markdown only.
- Markdown is formatted by the fixer of markdownlint-cli2, not by Prettier, so the formatter and the linter of Markdown cannot disagree.
- YAML is formatted by Prettier and linted by yamllint. The layout rules of yamllint are set to agree with Prettier, and line length belongs to Prettier alone (`.yamllint.yaml`).
- Shell style lives in `.editorconfig`, which shfmt reads, so editors and shfmt use the same settings.
- editorconfig-checker does not check the size of indentation, which belongs to the formatters.
- `terraform fmt` formats the files of the Terraform language, and `terragrunt hcl fmt` every other HCL file, so no file has two formatters.
- The choice of Trivy is in TPL-0024. One overlap is accepted: Trivy reports every vulnerable module in every lockfile, and govulncheck narrows the findings for Go to the code that is actually called, so a vulnerability in Go code that is called can be reported by both. Trivy can report the same module a second time in `scan-images`, which reads the modules that were compiled into each binary.
- The scanners on GitHub's side, CodeQL and secret scanning (TPL-0024), run outside `make`. They overlap with golangci-lint (gosec) and gitleaks on purpose: they are a second layer, which runs on GitHub whatever happens on a laptop.
- Commit messages and pull request titles are checked by `tools/scripts/commit-msg.sh`, a short script of this repository, not by a Node.js package such as commitlint or by a separate binary. A pinned binary needs a build for each of the four platforms of `mise.lock` (TPL-0022), and the binary that was chosen first, committed, publishes none for Linux on arm64; the script needs no dependency.
- OpenAPI and AsyncAPI documents are both linted with Redocly CLI. The first plan also named the AsyncAPI command-line tool, and Spectral as another choice. Neither is installed: one tool covers both kinds of contract, and the repository carries one large dependency tree fewer.
- The import rules of the layers and the rule against reading the clock in the domain live in `tools/cmd/arch`, not in depguard and forbidigo, which golangci-lint runs. The command derives every rule from the directory layout, so a new context needs no configuration. depguard and forbidigo keep only the rules that apply to every Go file.
- An import of a package under `internal/` of another context does not compile, so `lint-go` and `test-go` report it, and `lint-arch` reports it as well (`arch/context-import`). This overlap is accepted: only the finding of `lint-arch` names the record and says how contexts integrate instead (TPL-0005).
- squawk reads the SQL files and the DDL of each plan, with a configuration for each. The one for plans, `.squawk.plan.toml`, leaves the rules about what a table holds, such as the type of a column, to `lint-sql`, so such a problem is reported once, on `schema.sql`.

The rules for running the tools are as follows.

- `make check` has a budget of 60 seconds, which `tools/scripts/check.sh` enforces.
- After `make setup`, `make check` uses no network. `make setup` downloads the Go modules, fills the Go build cache, and installs the tflint plugin; lychee and zizmor run offline; pinact checks the pins without the GitHub API; check-jsonschema uses the schemas bundled with it; and Trivy uses the checks built into its binary.
- `make check` runs the checks in parallel, prints one line per check, and prints the findings of failed checks only. `VERBOSE=1` prints the output of every check.
- A check that is slow or needs the network goes to `make check-all`, which CI runs.
- Every target of `make check` and `make check-all` runs through `tools/scripts/task.sh`, which gives it the output format. A check that takes a list of files gets it from `tools/scripts/files.sh`.
- `.lintignore` is the one list that excludes vendored and generated files from the checks that take a list of files, and `files.sh` applies it. gitleaks and Trivy, which walk the tree themselves, skip dependency and cache directories on their own (`.gitleaks.toml`, `make/security.mk`). golangci-lint, which reads whole Go modules, skips generated Go files by their header (`.golangci.yml`).
- `CHECK_SCOPE` (`staged` or `changed`) and `CHECK_FILES` narrow every check that takes its files from `files.sh`, in the same way. When a change touches a tool pin or the definition of a check (`mise.toml`, a lockfile, `Makefile`, `make/`, `tools/scripts/`, `tools/go.mod`, or a configuration file at the root), every file is in scope, because such a change can alter the result for any file. lefthook and the hooks for agents use these variables, and CI uses them only for the slow checks of a pull request (TPL-0018).

### Consequences

- Good, because each problem is reported once, by the tool that owns it, except where Trivy and govulncheck overlap on purpose.
- Good, because `make check` is fast enough to run at the end of every agent turn and before every commit.
- Good, because a new check needs only a target in one `make/<area>.mk` and a case in the self-test, and it inherits the scope, the ignore list, the parallel run, and the output format.
- Bad, because choosing one tool per concern gives up the extra rules of the tools that were left out.
- Bad, because every tool has its own configuration file, and agreements between tools, such as the layout rules of yamllint and Prettier, must be kept by hand.
- Bad, because the budget pushes useful checks, such as known vulnerabilities and remote links, out of `make check`, so their findings appear only in CI, after a push.
- Bad, because the budget is measured on the machine that runs the check, so the same run can be within budget on a fast machine and over it on a slow one.

### Confirmation

- `tools/scripts/check.sh` fails `make check` when the run takes longer than `CHECK_BUDGET` (60 seconds), and its message names this record.
- Every target of `make check` and `make check-all` runs through `tools/scripts/task.sh`. A target that takes a list of files gets it from `tools/scripts/files.sh`, which applies `.lintignore`, `CHECK_SCOPE`, and `CHECK_FILES`.
- `make check-all` runs `selftest`, which fails when a target of `make check` has no case, or when a check no longer fails on its known-bad input.
- Review compares a new tool with the table in this record. A tool for a concern that already has a tool either replaces that tool, through a new record, or is not added.

## Pros and Cons of the Options

### One tool per concern, behind make, with a time budget

- Good, because each kind of finding has one source, in a known format.
- Good, because the same targets serve people, agents, the Git hooks, and CI.
- Bad, because the Makefile and its scripts are code that this repository must maintain.

### An aggregator such as MegaLinter or super-linter

- Good, because one container brings dozens of linters with little setup.
- Bad, because it runs several linters per language by default, with overlapping findings, and turning them off one by one is the same work as choosing.
- Bad, because it ships as a large container image whose tool versions follow the releases of the aggregator, not the pins of this repository.

### The pre-commit framework as the runner

- Good, because it is widely used and has hooks for most tools.
- Bad, because it installs each hook in an environment of its own, so the versions of tools would be pinned in a second place, apart from `mise.toml` and the lockfiles.
- Bad, because it is a second runner next to `make`, which CI and agents still need for everything that is not a hook, such as tests and scans.

### Every tool with its defaults, run only in CI

- Good, because there is no configuration to write.
- Bad, because people and agents learn about a problem only after a push.
- Bad, because the defaults of different tools overlap and disagree.

## More Information

- `make help` lists the main targets, and `make/<area>.mk` holds the targets of each area.
- The pins of these tools are in TPL-0022, and the choice of Trivy and of the scanners on GitHub's side in TPL-0024.
