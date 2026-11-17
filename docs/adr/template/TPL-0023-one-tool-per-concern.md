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

- Each finding comes from exactly one tool.
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

Each concern has exactly one tool. Targets marked "check-all" run only in `make check-all`.

| Concern | Tool | Target |
|---|---|---|
| Formatting of Go code | golangci-lint, with gofmt and goimports as formatters | `fmt-check-go` |
| Linting of Go code, including `go vet` | golangci-lint | `lint-go` |
| Go tests | `go test -race -shuffle=on` | `test-go` |
| Known vulnerabilities in Go code that is actually called | govulncheck | `vuln-go` (check-all) |
| Formatting of YAML, JSON, JSON5, JavaScript, TypeScript, and CSS | Prettier | `fmt-check-prettier` |
| Linting of YAML, such as duplicate keys and ambiguous values | yamllint | `lint-yaml` |
| Formatting and linting of TOML | taplo | `fmt-check-toml`, `lint-toml` |
| Configuration files against their schemas | check-jsonschema | `lint-schema`, `lint-schema-remote` (check-all) |
| Formatting of shell scripts | shfmt | `fmt-check-shell` |
| Linting of shell scripts | shellcheck | `lint-shell` |
| Linting of Dockerfiles | hadolint | `lint-dockerfile` |
| Syntax of workflows | actionlint | `lint-actions` |
| Security of workflows | zizmor | `lint-actions-security` |
| Policy of workflows: permissions and timeouts | ghalint | `lint-actions-policy` |
| Pins of actions to full commit SHAs | pinact | `lint-actions-pins` |
| Formatting and structure of Markdown | markdownlint-cli2 | `lint-md` |
| English prose | Vale, with the Google style | `lint-prose-en` |
| Japanese prose | textlint | `lint-prose-ja` |
| Spelling | typos | `lint-typos` |
| Links | lychee | `lint-links`, `lint-links-remote` (check-all) |
| Line endings, final newlines, trailing spaces, and the style of indentation | editorconfig-checker | `lint-editorconfig` |
| Secrets | gitleaks | `lint-secrets` |
| Misconfiguration of Dockerfiles and Terraform | Trivy | `scan-config` |
| Vulnerabilities and licenses of dependencies | Trivy | `scan-deps` (check-all) |
| Commit messages | committed | the `commit-msg` hook of lefthook |
| OpenAPI and AsyncAPI documents | Redocly (`@redocly/cli`) | none until the first contract exists |

Some tools are pinned already and get their targets when their area arrives: Terraform, Terragrunt, tflint, and terraform-docs for the infrastructure, squawk for SQL, and octocov for test coverage.

These overlaps were resolved as follows.

- `go vet` runs inside golangci-lint, together with the other Go linters, and is not a separate step. gofmt and goimports run there too, as formatters.
- Spelling belongs to typos, so the spelling rule of Vale is off (`.vale.ini`).
- Vale reads English Markdown only, and textlint reads Japanese Markdown only.
- Markdown is formatted by the fixer of markdownlint-cli2, not by Prettier, so the formatter and the linter of Markdown cannot disagree.
- YAML is formatted by Prettier and linted by yamllint. The layout rules of yamllint are set to agree with Prettier, and line length belongs to Prettier alone (`.yamllint.yaml`).
- Shell style lives in `.editorconfig`, which shfmt reads, so editors and shfmt use the same settings.
- editorconfig-checker does not check the size of indentation, which belongs to the formatters.
- Trivy covers misconfiguration, vulnerabilities, licenses, and later the SBOMs, so there is no second scanner. The first plan named syft as another choice for SBOMs.
- Commit messages are checked by committed, a single binary, not by a Node.js package such as commitlint.
- OpenAPI and AsyncAPI documents are both linted with Redocly. The first plan also named the AsyncAPI command-line tool, and Spectral as another choice. Neither is installed: one tool covers both kinds of contract, and the repository carries one large dependency tree fewer.

The rules for running the tools are as follows.

- `make check` has a budget of 60 seconds, which `tools/scripts/check.sh` enforces.
- `make check` uses no network: lychee and zizmor run offline, check-jsonschema uses the schemas bundled with it, and Trivy uses the checks built into its binary.
- `make check` runs the checks in parallel, prints one line per check, and prints the findings of failed checks only. `VERBOSE=1` prints the output of every check.
- A check that is slow or needs the network goes to `make check-all`, which CI runs.
- `.lintignore` is the one list that excludes vendored and generated files from every check. `tools/scripts/files.sh` applies it.
- `CHECK_SCOPE` (`staged` or `changed`) and `CHECK_FILES` narrow every target in the same way. lefthook and the hooks for agents use them.

### Consequences

- Good, because each problem is reported once, by the tool that owns it.
- Good, because `make check` is fast enough to run at the end of every agent turn and before every commit.
- Good, because a new tool needs only a target in one `make/<area>.mk`, and it inherits the scope, the ignore list, the parallel run, and the output format.
- Bad, because choosing one tool per concern gives up the extra rules of the tools that were left out.
- Bad, because every tool has its own configuration file, and agreements between tools, such as the layout rules of yamllint and Prettier, must be kept by hand.
- Bad, because the budget pushes useful checks, such as known vulnerabilities and remote links, out of `make check`, so their findings appear only in CI, after a push.
- Bad, because the budget is measured on the machine that runs the check, so the same run can be within budget on a fast machine and over it on a slow one.

### Confirmation

- `tools/scripts/check.sh` fails `make check` when the run takes longer than `CHECK_BUDGET` (60 seconds), and its message names this record.
- Every check target runs through `tools/scripts/task.sh` and `tools/scripts/files.sh`, which give it the output format, `.lintignore`, `CHECK_SCOPE`, and `CHECK_FILES`.
- Review compares a new tool with the table in this record. A tool for a concern that already has a tool either replaces that tool, through a new record, or is not added.

## Pros and Cons of the Options

### One tool per concern, behind make, with a time budget

- Good, because every finding has one source, in a known format.
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
- The pins of these tools are in TPL-0022, and the reasons for Trivy as the only scanner are in TPL-0024.
