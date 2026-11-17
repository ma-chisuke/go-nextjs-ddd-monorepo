---
status: accepted
date: 2026-11-17
---

# TPL-0010: Manage the schema declaratively with pgschema

## Context and Problem Statement

Each context owns one PostgreSQL schema (TPL-0009), and that schema changes as the context grows.

The common way to change a schema is a sequence of migration files, each holding the DDL of one step.
The current shape of a table is then written nowhere: to learn it, a reader replays the sequence in their head, or opens a database.
Agents have the same problem, and they write many of the changes.

The template wants the opposite.
The current schema of a context is written in one file that people and agents read, change, and review, and a tool works out the DDL that brings a database to that state.
The query generator reads the same file (TPL-0011), so the queries are checked against the schema that actually runs.

A schema change can lock a busy table or drop data, so the exact DDL must be visible before it runs, to a person and to a linter.

How do we describe the schema of each context, and how do we change a database to match it?

## Decision Drivers

- The current schema of a context is readable in one file, without a database.
- The DDL of a change is visible and reviewable before it runs, and a linter can check it.
- The tool works on one PostgreSQL schema at a time, because a context owns one schema (TPL-0009).
- The tool is open source and needs no account, because the template must work for anyone who copies it.
- The query generator can read the same file (TPL-0011).

## Considered Options

- pgschema: a desired state in SQL, with plan and apply
- Versioned migrations (golang-migrate, goose, or Flyway)
- Atlas, used declaratively
- sqldef
- Automatic migration by an ORM

## Decision Outcome

Chosen option: "pgschema: a desired state in SQL, with plan and apply", because it keeps the desired schema of a context as plain SQL in one file, shows the DDL of every change as a plan, and works on one schema at a time, without an account.

The rules are as follows.

- The desired schema of a context is `backend/internal/<context>/internal/adapter/postgres/schema.sql`. It holds the tables, indexes, constraints, and comments of that context's schema, and nothing outside it.
- `pgschema plan` compares the database with `schema.sql` and computes the DDL that turns the database into that state. `pgschema apply` runs the DDL. The owner role of the context runs both (TPL-0009).
- There are no migration history files. The history of the schema is the Git history of `schema.sql`.
- The DDL of every plan is linted with squawk and reviewed in the pull request that changes `schema.sql`.
- pgschema does not create schemas, roles, or extensions. `db/bootstrap.sql` does (TPL-0009).
- A change that must move or rewrite data, such as a backfill, is a separate, explicit step, reviewed on its own. A comparison of two schemas cannot know what should happen to the data.
- A rename is done in steps: add the new column or table, copy the data, move the code, and then drop the old one. A plan compares two states, so a rename in `schema.sql` looks like a drop and an add, and the drop loses the data.

### Consequences

- Good, because `schema.sql` is the one readable source of truth for a context's schema, and sqlc reads the same file (TPL-0011), so the schema that the database runs and the schema that the queries are checked against cannot drift apart.
- Good, because a reviewer sees both the intent (the diff of `schema.sql`) and the exact DDL (the plan).
- Good, because no sequence of migration files grows over the years, and two pull requests that change the schema meet as edits to one file, with no order of files to get right.
- Bad, because data migrations and backfills get no help from pgschema: each one is written, reviewed, and run as a separate step.
- Bad, because a rename needs care and several changes, as described in the rules.
- Bad, because the DDL depends on the database that the plan compares with. If an environment has drifted from the main branch, the DDL that runs there differs from the DDL that was reviewed.
- Bad, because pgschema is a young tool, sponsored by one company (Bytebase), with a small community. If it stalls, the template needs another tool. `schema.sql` is plain SQL, so the file itself survives such a change.

### Confirmation

squawk is pinned in `mise.toml`, and the target `lint-sql` of `make check` runs it on the SQL files in the repository.
M0 has no `schema.sql` yet, so that check has nothing to lint yet.

The checks that rely on a plan arrive with the first bounded context: CI computes the plan of each changed `schema.sql`, lints its DDL with squawk, and shows the plan in the pull request.
Until then, review enforces the decision.

## Pros and Cons of the Options

### pgschema: a desired state in SQL, with plan and apply

- Good, because the desired schema is plain SQL in one file, readable without the tool.
- Good, because the workflow is the plan and apply that Terraform users know, and apply checks that the database has not changed since the plan.
- Good, because it works on one schema at a time, which matches one schema per context (TPL-0009).
- Good, because it is open source (Apache 2.0) and needs no account.
- Bad, because it is young and has a small community.
- Bad, because it is not yet verified that it handles every object that the template needs (see More Information).

### Versioned migrations (golang-migrate, goose, or Flyway)

- Good, because they are mature and widely known.
- Good, because each step is explicit, so a data migration sits next to its DDL in the same sequence, and what runs is exactly what was reviewed.
- Bad, because the current schema is written nowhere: a reader replays the files, or reads a database or a dump that can be out of date.
- Bad, because two pull requests that each add a migration can merge cleanly and still conflict when both run, and the order of the files decides the outcome.
- Bad, because the files pile up, and squashing them is manual work.

### Atlas, used declaratively

- Good, because it is mature, offers both a declarative and a versioned workflow, and reads a desired state written in SQL or in its own HCL.
- Good, because its PostgreSQL driver supports exclusion constraints in every edition (as of November 2026).
- Bad, because, as of November 2026, parts of it need an Atlas account and the paid Pro plan: for PostgreSQL, views, functions, triggers, and extensions, and features such as migration linting. A template must work for anyone who copies it, without an account.

### sqldef

- Good, because it is declarative like pgschema, reads plain SQL, previews the DDL with a dry run, and has been used in production for years.
- Good, because it supports several databases, and it can rename a column when the desired schema marks the rename.
- Bad, because it reads SQL with a parser of its own, shared by several databases, not with the parser of PostgreSQL, so it can lag behind PostgreSQL syntax or reject it.

### Automatic migration by an ORM

- Good, because the schema follows the code, with no extra file.
- Bad, because the template uses no ORM (TPL-0011), and the domain must not know about the database (TPL-0004).
- Bad, because automatic migration changes the database when the program starts, with no DDL to review, and it usually only adds: for example, GORM's `AutoMigrate` does not drop columns that the code no longer uses.

## More Information

- Not yet verified: whether pgschema handles the exclusion constraint that the sample needs, `EXCLUDE USING gist (room_id WITH =, period WITH &&)`. As of November 2026, the documentation of pgschema lists primary key, foreign key, unique, and check constraints, and does not mention exclusion constraints. This is checked with the first bounded context. If pgschema cannot handle it, a new record decides what to do.
- [pgschema](https://www.pgschema.com/) and its [repository](https://github.com/pgplex/pgschema)
- [squawk](https://squawkhq.com/), the linter for PostgreSQL migrations
- [Features of Atlas](https://atlasgo.io/features), including which features are in the Pro plan
