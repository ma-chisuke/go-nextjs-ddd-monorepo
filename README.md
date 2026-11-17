# go-nextjs-ddd-monorepo

A template for a product with a Go API and a Next.js front end in one repository, split by the bounded contexts of domain-driven design, and built so that coding agents and people work under the same checks.

[日本語](README.ja.md)

## Status

The template is built in milestones. The quality foundation comes first, so that every line of app code is checked from its first commit.

| Milestone | Content | State |
|---|---|---|
| M0 | Quality foundation: pinned tools, `make check`, formatters and linters, security scans, CI, Git hooks, agent hooks, decision records | Done |
| M1 | One core and one supporting bounded context, from OpenAPI through the use case and the domain to PostgreSQL | In progress |
| M2 | Events: outbox, EventBridge, SQS, and the local AWS emulator | Planned |
| M3 | Next.js front end and end-to-end tests | Planned |
| M4 | Terraform and Terragrunt for stg and prd, and deployment | Planned |
| M5 | Evaluation of the agent harness, property-based API tests, mutation tests | Planned |

The sample business is meeting-room booking, with three bounded contexts: Facility, Reservation, and Notification.

## What the template decides for you

Each decision has a record with its reasons and the options that lost. The full list is in [docs/adr/](docs/adr/README.md).

- **Architecture.** Bounded contexts that depend on each other only through HTTP APIs and events. Clean architecture inside a context, with a domain layer that does no I/O.
- **Contracts.** OpenAPI first. Go server code and TypeScript types are generated, and several layers of checks keep the document and the code in agreement. Errors are RFC 9457 problem details.
- **Data.** One PostgreSQL 18 database with a schema and two roles per context, a declarative schema, and typed queries.
- **AWS.** ECS per context, Aurora, EventBridge and SQS, Terraform and Terragrunt, separate accounts for stg and prd, and a local environment that needs no AWS account.
- **Quality.** One tool per concern behind `make check`, pinned tool versions with checksums, supply chain defenses, and CI that runs the same commands as a laptop.
- **Agents.** Rules are enforced by checks that print what to fix. `AGENTS.md` stays short.
- **Language.** English is the source, and every document has a Japanese translation next to it.

## Get started

You need Git, GNU Make, and [mise](https://mise.jdx.dev/installing-mise.html). mise installs every other tool at the version that `mise.toml` pins.

```sh
make setup   # tools, dependencies, and Git hooks
make check   # fast checks; takes a few seconds
```

| Command | Use |
|---|---|
| `make setup` | Install the pinned tools, the dependencies, and the Git hooks. |
| `make fmt` | Format files. |
| `make gen` | Regenerate generated files. |
| `make check` | Fast checks, without the network. Run it before every commit. |
| `make check-all` | Fast and slow checks, including those that need the local PostgreSQL. CI runs this. |
| `make db-up` | Start the local PostgreSQL in Docker, and apply the schema of every context. |
| `make help` | List the main targets. |

A dev container in `.devcontainer/` gives the same environment without installing anything on your machine.

The local database and `make check-all` need Docker. [Run the services locally](docs/guides/run-locally.md) starts the database and the two services, and books a room through their APIs with curl.

## Use the template

1. Create a repository from this template on GitHub.
2. Clone it, and run `make setup`.
3. Run `make init MODULE=github.com/<owner>/<repository> NAME=<project>` to replace the module path and the project name.
4. Apply the repository settings with the [GitHub settings guide](docs/guides/github-settings.md).
5. Record your own decisions in `docs/adr/`. Leave `docs/adr/template/` as it is, so that you can compare your copy with later versions of the template.

## What `make check` covers

| Area | Checks |
|---|---|
| Every file | Spelling, EditorConfig rules, secrets |
| Go | Formatting, golangci-lint (including go vet), tests with the race detector; known vulnerabilities and the integration tests against PostgreSQL in `make check-all` |
| Architecture of the backend | `lint-arch`: imports between contexts and layers, a domain without I/O, routes only from the API documents, schemas named in SQL, and the glossary of each context |
| Generated code | The code generated from the API documents and from the SQL of each context is what the generators write now |
| TypeScript and Next.js | Prettier, ESLint with import boundaries, type check, unused code |
| Shell and Dockerfiles | shfmt, shellcheck, hadolint |
| GitHub Actions | actionlint, zizmor, ghalint, pinact |
| Terraform and Terragrunt | Formatting, tflint, Trivy; validation and module documents in `make check-all` |
| SQL | squawk; in `make check-all`, squawk on the DDL of each schema change (`lint-sql-plan`), every query prepared against its schema (`lint-sqlc`), and the comments and documents of each schema (`lint-db-docs`) |
| OpenAPI and AsyncAPI | Redocly; breaking changes against the base branch with oasdiff (`lint-contracts-breaking`) |
| YAML, JSON, and TOML | Prettier, yamllint, taplo, JSON Schema validation of configuration files |
| Markdown | markdownlint, links, English style (Vale), Japanese style (textlint) |
| Documents | Format of decision records, their index, and translations that match their English source |
| Dependencies | Vulnerabilities and licenses with Trivy, in `make check-all` |

`make check` has a time budget of 60 seconds and uses no network. Every finding is one line with the file, the line, and the rule.

## Documents

- [ARCHITECTURE.md](ARCHITECTURE.md): the map of the backend: its contexts, layers, and database, and the checks that hold them in place.
- [docs/contexts/](docs/contexts/): the README and the glossary of each bounded context.
- [AGENTS.md](AGENTS.md): instructions for coding agents.
- [docs/adr/](docs/adr/README.md): decision records.
- [docs/guides/](docs/guides/): how to do recurring work.
- [SECURITY.md](SECURITY.md): how to report a vulnerability.

## Third-party files

- `.vale/styles/Google/`: the Google developer documentation style for Vale, under the MIT License.
- `.claude/skills/japanese-tech-writing/`: a writing norm for Japanese technical documents by Keiichiro Shikano, under the Unlicense.

## License

[MIT No Attribution](LICENSE). You can copy the template and make it your own without keeping a notice.
