# AGENTS.md

Instructions for coding agents. People start with [README.md](README.md).

This file holds commands, where things go, and the rules that differ from common defaults.
The reason for a rule is in the decision record that the rule cites ([docs/adr/](docs/adr/README.md)).

## Commands

| Command | Use |
|---|---|
| `make setup` | Install the pinned tools, the dependencies, and the Git hooks. |
| `make fmt` | Format files. |
| `make gen` | Regenerate generated files. |
| `make check` | Fast checks. A change is not finished until this passes. |
| `make check-all` | Fast and slow checks. CI runs this. |

- Narrow a run while you work: `CHECK_SCOPE=changed make check` or `CHECK_FILES=<path> make check`. Run the full `make check` at the end.
- A failed check prints `file:line: [rule] message`. Fix the cause. Do not turn the rule off, add an ignore comment, or exclude the file.
- If a rule is wrong for your case, stop and say so. Changing a decision needs a decision record (skill `write-adr`).

## Where things go

| Path | Content |
|---|---|
| `backend/` | Go API, one module. `internal/<context>/` per bounded context. Read [backend/AGENTS.md](backend/AGENTS.md) before you change it. |
| `frontend/` | Next.js. `src/features/<context>/` per bounded context. Read [frontend/AGENTS.md](frontend/AGENTS.md) before you change it. |
| `contracts/` | OpenAPI documents in `http/`; AsyncAPI documents and the JSON Schema of each event in `events/`. The source of generated code and of the contract tests. |
| `db/` | Database bootstrap. |
| `infra/` | Terraform: `modules/`; `live/`, the units of stg and prd that Terragrunt runs ([guide](docs/guides/deploy.md)); and `local/`, which applies modules to AWS Emulator for the local system. |
| `tools/` | Checks and generators of this repository (Go module, scripts). |
| `make/` | One file of make targets per area. |
| `docs/adr/` | Decision records. `template/` belongs to the template. |
| `docs/contexts/` | The README and the glossary of each bounded context. |
| `docs/problems/` | The page of each problem type that the APIs answer with. |
| `docs/guides/` | How to do recurring work, such as adding an endpoint or changing a schema. The skills in `.claude/skills/` follow them. |
| `evals/` | The evaluation of the harness: tasks, graders, and the records of the runs ([README](evals/README.md)). |
| `.claude/` | Hooks, skills, and subagents for Claude Code. |

## Rules that differ from common defaults

- **Tools.** Never install a tool globally, and never run one with `npx`, `go install`, or `pip`. Pin it in `mise.toml` (then `make tools-lock`), `package.json`, `pyproject.toml`, or `tools/go.mod`, and run it through `make`. (TPL-0022)
- **One tool per concern.** Before you add a linter or a formatter, check that no pinned tool covers the concern. (TPL-0023)
- **Documents.** `foo.md` is English and is the source. `foo.ja.md` is its Japanese translation. Change both in one commit, then run `make i18n-stamp`. Code, comments, and commit messages are English only. (TPL-0021, [guide](docs/guides/translation.md))
- **Decision records.** Do not edit `docs/adr/template/`. Record a new or changed decision in `docs/adr/ADR-NNNN-<slug>.md`. (TPL-0001)
- **Generated files and lockfiles.** Do not edit them by hand. Change the source and run `make gen`, or use the package manager. After a change to a `schema.sql`, also run `make gen-db-docs` (needs Docker).
- **Schemas.** Change only the `schema.sql` of a context; there are no migration files. Never add a line to `schema-changes.allow` on your own: letting a dangerous statement through is a decision of review. (TPL-0010)
- **Scanner exceptions.** Never add an entry to `.trivyignore.yaml` on your own: accepting a finding of Trivy is a decision of review. Change the code so that the finding goes away, or stop and say so. (TPL-0024)
- **Mutation tests.** Never add a line to `equivalent-mutants.allow` on your own: letting a mutant live is a decision of review. Kill a surviving mutant with a test, or make the code simpler. (TPL-0023, [guide](docs/guides/mutation-tests.md))
- **Commits.** Conventional Commits, one purpose per commit. Do not skip hooks.
- **GitHub Actions.** After checkout, the setup of mise, and the sign-in to AWS, a workflow step calls `make` and nothing else. Actions are pinned to a commit SHA. (TPL-0018, TPL-0024)
- **Environments.** `infra/live/stg/` and `infra/live/prd/` differ in `env.hcl` alone: change a unit in `infra/live/_units/`, for both. (TPL-0015)
- **Never** read `.env` files, force push, push to `main`, or apply infrastructure to prd.
