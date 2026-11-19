# Change a schema

The `schema.sql` of a context is the desired state of its schema (TPL-0010).
There are no migration files: pgschema compares a database with the file and plans the DDL that brings the database to it, and sqlc and tbls read the same file.

## Steps

1. Edit `backend/internal/<context>/internal/adapter/postgres/schema.sql`:

   - Write names without a schema. The search path of the roles of the context picks it.
   - Give a new table a primary key of type `uuid` with `DEFAULT uuidv7()`, unless the key is also a foreign key (TPL-0012).
   - Give every new table and column a comment, with `COMMENT ON TABLE` and `COMMENT ON COLUMN`.
   - Grant the runtime role its rights on a new table: `GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE <table> TO <context>_runtime;`. pgschema manages privileges, and the bootstrap sets no default privileges, so without this line the service cannot use the table.
   - Write an index as a plain `CREATE INDEX`. On a table that exists already, pgschema creates it with `CONCURRENTLY` by itself.

   Check: `make check` runs squawk on the file (`lint-sql`), and fails when the SQL names a schema (`arch/schema-reference`).

2. Apply the change to the local database. If PostgreSQL does not run yet, `make db-up` starts it and applies every schema. Otherwise:

   ```sh
   make db-plan    # print the DDL for each context; changes nothing
   make db-apply   # run it
   ```

   Check: `make db-plan` prints no statement afterwards.

3. Regenerate the typed queries:

   ```sh
   make gen
   ```

   sqlc reads the new schema, so a query that names a column that no longer exists fails here. Fix the queries in `queries/` and the code that uses the generated types: the store in a core context, the use cases in a supporting context, and the in-memory fakes.

   Check: `make check` (`gen-check-sqlc`, `lint-go`, and `test-go`).

4. Regenerate the schema documents in `docs/generated/db/<context>/`:

   ```sh
   make gen-db-docs
   ```

   `make gen` does not write them, because it needs no database.

   Check: `lint-db-docs` in `make check-all` fails on a table or a column without a comment, on a foreign key without an index, and on documents that differ from what tbls writes.

5. Run the checks that need PostgreSQL:

   ```sh
   CHECK_SCOPE=changed make check-all
   ```

   `lint-sql-plan` lints the DDL of the change, as the next section describes. `lint-sqlc` prepares every query against the new schema, and `test-integration` runs the integration tests, which also try the privileges of the runtime role.
   The plan stays in `.cache/sql-plan/<context>.sql`. CI does not show it in the pull request yet, so a reviewer runs the check and reads it there.

## What lint-sql-plan protects against

`lint-sql-plan` applies the `schema.sql` of the base commit to a scratch database, asks pgschema for the plan to the `schema.sql` of the change, and lints that DDL with squawk (`.squawk.plan.toml`).
The base commit is the merge base with `origin/main`, or with `CHECK_BASE` when it is set.
The DDL will run against a database that holds data and serves requests, so the check fails on statements such as these:

- `DROP COLUMN` and `DROP TABLE` (`ban-drop-column`, `ban-drop-table`): the data is lost, and the code that still runs may read it.
- A new `NOT NULL` column without a default (`adding-required-field`): the rows that exist cannot have a value for it.
- A new type for a column (`changing-column-type`): PostgreSQL rewrites the table under a lock that blocks reads and writes.
- A new `UNIQUE` constraint on a table that exists (`disallowed-unique-constraint`): the lock blocks reads and writes while the index is built.
- A new `CHECK` constraint on a table that exists (`constraint-missing-not-valid`): pgschema adds and validates it in one transaction.

A nullable column, a column with a constant default, and a new index pass.

## Make a change in safe steps

A change that the check refuses is usually a change to make in several steps. Merge and apply each step before the next, so that the code that runs during a step works with the schema before and after it (TPL-0010).

To rename a column:

1. Add the new column, nullable or with a default.
2. Copy the data. pgschema does not move data, so write that statement apart, review it, and run it as a step of its own.
3. Move the code to the new column, and deploy it.
4. Drop the old column.

A rule of uniqueness on a table that exists needs no steps: write `CREATE UNIQUE INDEX` instead of a `UNIQUE` constraint, and pgschema builds the index with `CONCURRENTLY`.

The last step of a sequence, such as the `DROP COLUMN` at the end of a rename, is still the dangerous statement that the steps before it prepared for, and the check fails on it.
When the steps before it are done, a person decides in review to let that one statement through. The change lists the statement in `backend/internal/<context>/internal/adapter/postgres/schema-changes.allow`, one line per statement:

```text
# <squawk rule> | <statement, exactly as the finding quotes it> | <reason>
ban-drop-column | ALTER TABLE rooms DROP COLUMN name | since release 0.4.0 the code reads display_name, and every row was copied to it
```

- Blank lines and lines that start with `#` are ignored.
- The reason is not optional: a line without one is a finding (`sql-plan/allowance-format`).
- A line lets through one statement of the current plan, with the same rule. A line that matches no statement of the plan is a finding as well (`sql-plan/unused-allowance`). Once the change is merged, the base commit holds the new schema, so the next change to that `schema.sql` plans no such statement and has to remove the line. Allowances do not pile up.

An allowance is not a way around the check: it records that a person reviewed one dangerous statement of one change, and why the statement is safe now.

## In AWS, the schema changes before the code

A deployment applies every `schema.sql` before it starts the new code ([Deploy to AWS](deploy.md#deploy)). The task that prepares the database runs pgschema as the owner role of each context while the services still run the old code, and only then does the deployment start the services with the new code. A change to `schema.sql` must therefore work with the code before it and with the code after it (TPL-0010).

- Add first. A new table, a nullable column, a column with a default, and an index work with the old code, which does not use them.
- Remove in a later deployment. Drop a column or a table only after a deployment of code that no longer uses it has reached every environment.
- A rule that the old code would break waits too. A `NOT NULL` column that the old code does not write, for example, is added nullable; the code that writes it is deployed; and a later change adds the constraint.

Each step of [Make a change in safe steps](#make-a-change-in-safe-steps) is therefore one merge into `main`, and one deployment. The deployment of prd waits for a reviewer, so check that a step reached prd before you merge the next one.

The task applies its plan without asking. What review reads is the plan of `lint-sql-plan`, against the schema of the base commit, which is the schema of an environment only when the environment runs that commit.
