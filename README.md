# go-nextjs-ddd-monorepo

A template for a product with a Go API and a Next.js front end in one repository, split by the bounded contexts of domain-driven design, and built so that coding agents and people work under the same checks.

[日本語](README.ja.md)

## Status

The template was built in milestones. The quality foundation came first, so that every line of app code was checked from its first commit. Every milestone is done; the table says what M4 and M5 leave open.

| Milestone | Content | State |
|---|---|---|
| M0 | Quality foundation: pinned tools, `make check`, formatters and linters, security scans, CI, Git hooks, agent hooks, decision records | Done |
| M1 | One core and one supporting bounded context, from OpenAPI through the use case and the domain to PostgreSQL | Done |
| M2 | Events: outbox, EventBridge, SQS, and the local AWS emulator, with the Notification context | Done |
| M3 | Next.js front end and end-to-end tests | Done |
| M4 | Terraform and Terragrunt for stg and prd, and deployment | Done, and not yet applied to AWS: the modules are tested against a mock of the AWS provider, every unit is validated, and stg is rehearsed on AWS Emulator |
| M5 | Evaluation of the agent harness, property-based API tests, mutation tests | Done, with one thing open: the evaluation has twenty runs, one or two for each arm of six tasks, a first look rather than a measurement |

The sample business is meeting-room booking, with three bounded contexts: Facility, Reservation, and Notification.

## What the template decides for you

Each decision has a record with its reasons and the options that lost. The full list is in [docs/adr/](docs/adr/README.md).

- **Architecture.** Bounded contexts that depend on each other only through HTTP APIs and events. Clean architecture inside a context, with a domain layer that does no I/O.
- **Contracts.** OpenAPI first. Go server code and TypeScript types are generated, and several layers of checks keep the document and the code in agreement. Errors are RFC 9457 problem details.
- **Data.** One PostgreSQL 18 database with a schema and two roles per context, a declarative schema, and typed queries.
- **AWS.** ECS per context, with private APIs and one public load balancer, Aurora, EventBridge and SQS, Terraform and Terragrunt, separate accounts for stg and prd, deployments from GitHub Actions through OIDC, and a local environment that needs no AWS account.
- **Quality.** One tool per concern behind `make check`, pinned tool versions with checksums, supply chain defenses, and CI that runs the same commands as a laptop.
- **Agents.** Rules are enforced by checks that print what to fix. `AGENTS.md` stays short, and gains an instruction only when the evaluation in `evals/` shows a failure that no check can catch.
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
| `make check-all` | Fast and slow checks, including those that need the local PostgreSQL and AWS Emulator. CI runs this. |
| `make up` | Start the local system in Docker: PostgreSQL with every schema, AWS Emulator with the event bus and the queues, Mailpit, every process of every context, and the frontend at <http://127.0.0.1:13000>. `make down` stops it. |
| `make db-up` | Start the local PostgreSQL in Docker, and apply the schema of every context. |
| `make infra-plan ENV=stg` | Plan every unit of an environment in AWS, with the credentials of its account. |
| `make deploy ENV=stg` | Push the images that changed, prepare the database, and apply every unit of an environment. prd deploys only from the workflow `Deploy`. |
| `make infra-bootstrap ENV=stg` | Once for each AWS account, as an administrator: create the bucket of the Terraform state and the roles of GitHub Actions. |
| `make infra-lock` | Write the checksums of the providers after a change to `infra/live/providers.hcl`. |
| `make help` | List the main targets. |

A dev container in `.devcontainer/` gives the same environment without installing anything on your machine.

The local system and `make check-all` need Docker. After `make up`, open <http://127.0.0.1:13000> to register a room and book it. [Run the services locally](docs/guides/run-locally.md) also books a room through the APIs with curl, and shows the mail that a booking causes in Mailpit.

| Service | Address |
|---|---|
| The frontend | <http://127.0.0.1:13000> |
| Mailpit, the mail of the local system | <http://127.0.0.1:18025> |
| The APIs of Facility and Reservation | <http://127.0.0.1:18081>, <http://127.0.0.1:18082> |
| PostgreSQL, AWS Emulator | `127.0.0.1:15432`, `https://localhost:19297` |

## Use the template

1. Create a repository from this template on GitHub.
2. Clone it, and run `make setup`.
3. Run `make init MODULE=github.com/<owner>/<repository> NAME=<project>` to replace the module path and the project name.
4. Apply the repository settings with the [GitHub settings guide](docs/guides/github-settings.md).
5. To deploy, prepare an AWS account for stg and one for prd with [Deploy to AWS](docs/guides/deploy.md). Until then, the workflows that reach AWS are skipped.
6. Record your own decisions in `docs/adr/`. Leave `docs/adr/template/` as it is, so that you can compare your copy with later versions of the template.

## What `make check` covers

| Area | Checks |
|---|---|
| Every file | Spelling, EditorConfig rules, secrets |
| Go | Formatting, golangci-lint (including go vet), tests with the race detector; in `make check-all`, known vulnerabilities, the integration tests against PostgreSQL and AWS Emulator, a smoke test of the whole local system, and mutation tests of the domain and the use cases of the core context (`test-mutation`) |
| Architecture of the backend | `lint-arch`: imports between contexts and layers, a domain without I/O, routes only from the API documents, schemas named in SQL, and the glossary of each context |
| Generated code | The Go code generated from the API documents and from the SQL of each context, and the TypeScript types generated from the API documents (`gen-check-api-ts`), are what the generators write now |
| TypeScript and Next.js | Prettier, ESLint with import boundaries, type check, unused code, unit tests on the test runner of Node (`test-frontend`), a scenario in a browser with each new page (`lint-new-pages`); in `make check-all`, the end-to-end test in a browser with Playwright (`e2e`) and mutation tests of the pure logic (`test-mutation-frontend`) |
| Shell and Dockerfiles | shfmt, shellcheck, hadolint |
| GitHub Actions | actionlint, zizmor, ghalint, pinact |
| Terraform and Terragrunt | Formatting, tflint, Trivy, the module READMEs (terraform-docs), and the same units in stg and prd (`lint-infra-live`); in `make check-all`, the validation and the tests of each module (`validate-terraform`), the validation of every unit (`validate-live`), and the rehearsal of stg on AWS Emulator (`rehearse-infra`) |
| SQL | squawk; in `make check-all`, squawk on the DDL of each schema change (`lint-sql-plan`), every query prepared against its schema (`lint-sqlc`), the comments and documents of each schema (`lint-db-docs`), and the image that prepares the database in AWS (`test-db-image`) |
| OpenAPI and AsyncAPI | Redocly; breaking changes against the base branch with oasdiff (`lint-contracts-breaking`); in `make check-all`, requests that Schemathesis generates from each OpenAPI document, sent to the APIs built from the checkout (`test-api-fuzz`) |
| JSON Schemas of events | Their form, their examples, and their messages in the AsyncAPI documents (`lint-events`); breaking changes against the base branch (`lint-events-breaking`) |
| YAML, JSON, and TOML | Prettier, yamllint, taplo, JSON Schema validation of configuration files |
| Markdown | markdownlint, links, English style (Vale), Japanese style (textlint) |
| Documents | Format of decision records, their index, and translations that match their English source |
| Agent harness | The hooks, fed sample events (`test-hooks`), the length of `AGENTS.md`, and the comments that turn a check off, which only review lets in (`lint-ignore-comments`); in `make check-all`, the graders of the evaluation, proven without an agent (`test-evals`) |
| Dependencies | Vulnerabilities and licenses with Trivy, in `make check-all` |

`make check` has a time budget of 60 seconds, 90 in CI, and uses no network. Every finding is one line with the file, the line, and the rule.

## Documents

- [ARCHITECTURE.md](ARCHITECTURE.md): the map of the product: its contexts and their processes, layers, the frontend, the path of a request from a screen and of an event, the database, the environments in AWS, and the checks that hold them in place.
- [docs/contexts/](docs/contexts/): the README and the glossary of each bounded context.
- [AGENTS.md](AGENTS.md): instructions for coding agents, with those for the backend and the frontend in `backend/AGENTS.md` and `frontend/AGENTS.md`.
- [docs/adr/](docs/adr/README.md): decision records.
- [docs/guides/](docs/guides/): how to do recurring work.
- `evals/README.md`: the evaluation of the agent harness, and what its runs have shown so far.
- [SECURITY.md](SECURITY.md): how to report a vulnerability.

## Third-party files

- `.vale/styles/Google/`: the Google developer documentation style for Vale, under the MIT License.
- `.claude/skills/japanese-tech-writing/`: a writing norm for Japanese technical documents by Keiichiro Shikano, under the Unlicense.

## License

[MIT No Attribution](LICENSE). You can copy the template and make it your own without keeping a notice.
