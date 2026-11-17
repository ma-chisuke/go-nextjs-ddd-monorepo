---
status: accepted
date: 2026-11-17
---

# TPL-0011: Access the database with sqlc

## Context and Problem Statement

Each context reads and writes the tables of its own schema (TPL-0009).
The code that does this, in the adapter of the context, is where SQL meets Go types.
Mistakes there, such as a misspelled column, a wrong type, or a NULL that the Go type cannot hold, show up only when the query runs, unless something checks them earlier.

Three other decisions constrain this code.
A context must not query the tables of another context (TPL-0009).
The domain must not know about the database (TPL-0004), so rows become domain objects in the adapter, not in the domain.
And agents write much of the SQL, so a query must be checked by a machine before it reaches review.

How does the Go code of a context run SQL against its schema?

## Decision Drivers

- A query that does not match the schema fails before it runs: at generation or at compilation.
- The SQL that runs is the SQL that a reviewer reads. No layer writes SQL out of sight.
- A context can query only its own schema.
- The result is plain Go over the pgx driver, with no framework at run time.

## Considered Options

- sqlc: hand-written SQL, with typed Go generated for pgx
- An ORM (GORM or ent)
- A query builder (for example squirrel or goqu)
- Hand-written code over pgx

## Decision Outcome

Chosen option: "sqlc: hand-written SQL, with typed Go generated for pgx", because it checks every query against the schema of the context when it generates code, and gives typed Go, while the SQL stays plain and readable.

The rules are as follows.

- SQL is written by hand in `backend/internal/<context>/internal/adapter/postgres/queries/*.sql`, with a name for each statement.
- sqlc generates typed Go for pgx (version 5) from those queries. `make gen` runs it, and nobody edits the generated code by hand.
- sqlc is configured per context. The configuration of a context reads only that context's `schema.sql` (TPL-0010) and that context's queries.
- The generated code stays in the adapter. In a core context, a repository in the adapter turns the generated rows into domain objects, so generated types never reach the domain or the use cases (TPL-0004). In a supporting context, which has no domain layer, the use cases call the generated queries directly (TPL-0003).
- Generated Go files end in `.gen.go`, so that `.gitattributes` and `.lintignore` treat them as generated code, as with the code generated from OpenAPI (TPL-0006). The options of sqlc for the names of its output files, such as `output_db_file_name`, `output_models_file_name`, and `output_files_suffix`, set these names. Not yet verified: that these options cover every file that sqlc writes. This is checked with the first bounded context.

### Consequences

- Good, because a query that names a missing column, or that passes a value of the wrong type, fails at generation, before any test runs.
- Good, because a query on a table of another context fails at generation: that table is not in the schema that sqlc reads.
- Good, because the SQL in review is the SQL that runs, and `EXPLAIN` works on it as written.
- Bad, because dynamic queries, such as a search with optional filters or a sort order chosen by the caller, are awkward. Each shape becomes a separate query, or one query with conditions such as `sqlc.narg('status') IS NULL OR status = sqlc.narg('status')`, which PostgreSQL can plan poorly.
- Bad, because the generated code lives in the repository, so a change to the schema or to a query also changes generated files in the same pull request.
- Bad, because sqlc reads SQL with the parser of one PostgreSQL release (libpg_query), and keeps its own list of types and functions. Syntax or a function that is new in PostgreSQL 18 can fail in sqlc until sqlc catches up.

### Confirmation

The following checks enforce this decision.

- `make check` fails when the generated code differs from what `make gen` writes.
- The target `lint-sqlc` of `make check-all` runs `sqlc vet` for each context in a scratch database of the local PostgreSQL that holds the `schema.sql` of the context. The rule `sqlc/db-prepare` prepares every query in PostgreSQL, and the rule `no-unfiltered-write` fails an `UPDATE` or `DELETE` without `WHERE`. These settings are the same in the `sqlc.yaml` of every context, and the check fails when one lacks `sqlc/db-prepare`.
- A query on a table of another context fails at generation, because that table is not in the schema that sqlc reads.

## Pros and Cons of the Options

### sqlc: hand-written SQL, with typed Go generated for pgx

- Good, because queries are checked against the schema at generation, and the output is plain Go that needs nothing at run time except pgx.
- Good, because it reads `schema.sql` directly, the same file that pgschema applies (TPL-0010).
- Bad, because dynamic queries are awkward, and its parser can lag behind new PostgreSQL syntax.

### An ORM (GORM or ent)

- Good, because dynamic queries and relations are easy to write, and much of the code either comes from a generator (ent) or is not needed at all (GORM).
- Bad, because the SQL that runs is out of sight, so it is easy to load too much or to send one query per row without noticing.
- Bad, because GORM checks queries only at run time, and ent defines the schema in Go, which conflicts with `schema.sql` as the one source of the schema (TPL-0010).
- Bad, because the models of an ORM tempt a team to use them as domain objects, which ties the domain to the database (TPL-0004).

### A query builder (for example squirrel or goqu)

- Good, because dynamic queries are natural to write.
- Bad, because nothing checks a column name or a type against the schema before the query runs.
- Bad, because a reviewer reads Go code that assembles SQL, not the SQL itself.

### Hand-written code over pgx

- Good, because there is nothing to generate and nothing hidden, and pgx is the driver in every case.
- Bad, because scanning rows into structs by hand is repetitive, and a mismatch between a query and its scanning code shows up only when the query runs.
- Bad, because nothing stops a query on another context's table before run time; only the database role stops it then (TPL-0009).

## More Information

- Not yet verified: whether sqlc parses every construct that the schemas and queries of the template use with PostgreSQL 18. This is checked with the first bounded context.
- [sqlc documentation](https://docs.sqlc.dev/)
- [pgx](https://github.com/jackc/pgx), the PostgreSQL driver for Go
