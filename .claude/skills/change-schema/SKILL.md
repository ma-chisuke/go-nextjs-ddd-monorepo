---
name: change-schema
description: Change the database schema of a bounded context, its schema.sql, and the queries and documents that follow from it. Use when a change adds, alters, or drops a table, a column, an index, or a constraint, or when lint-sql-plan reports a finding.
---

# Change a schema

Follow `docs/guides/change-a-schema.md`. It has the steps, what `lint-sql-plan` refuses, and how to make such a change in safe steps; this skill adds only what to verify.

## Steps

1. Edit only `backend/internal/<context>/internal/adapter/postgres/schema.sql`. There are no migration files, and pgschema computes the DDL.
2. Follow the guide: `make db-plan` and `make db-apply` on the local database, `make gen`, and `make gen-db-docs`. Never edit `*.gen.go` or `docs/generated/`.
3. Verify, before you finish:
   - every new table has a `GRANT SELECT, INSERT, UPDATE, DELETE` to `<context>_runtime`, and a comment on itself and on each column;
   - no statement names a schema;
   - a statement that `lint-sql-plan` refuses is split into safe steps, as the guide describes. Do not write `schema-changes.allow` on your own: an allowance is a decision of the person who reviews, so stop and ask, with the finding and the reason.
4. Run `make fmt` and `CHECK_SCOPE=changed make check-all`, which needs Docker and runs `lint-sql-plan`, `lint-sqlc`, `lint-db-docs`, and the integration tests. Finish with `make check`.
