# SQL: squawk on every SQL file, with the rules that suit declarative schema
# files (.squawk.toml). In check-all, because they need PostgreSQL: squawk on
# the DDL of each schema change, and sqlc vet on the queries of each context.

CHECK_TARGETS += lint-sql
CHECK_ALL_TARGETS += lint-sql-plan lint-sqlc

DB_CHECK := tools/scripts/db-check.sh

lint-sql:
	@$(TASK) $@ '$(FILES) "*.sql" | xargs -r squawk --config=.squawk.toml --reporter=json \
		| jq -r -f tools/squawk/findings.jq'

# The DDL that pgschema plans from the schema.sql of the base commit (the
# merge base with origin/main, or with CHECK_BASE) to the one of the working
# tree, linted with the rules for a running migration (.squawk.plan.toml,
# TPL-0010). In a scratch database; see tools/scripts/db-check.sh.
lint-sql-plan: | db-server
	@$(TASK) $@ '$(FILES) "backend/internal/*/schema.sql" "db/*.sql" | $(DB_CHECK) plan'

# Every query prepared in a scratch database that holds the schema of its
# context, and the rules of each sqlc.yaml (TPL-0011).
lint-sqlc: | db-server
	@$(TASK) $@ '$(FILES) "backend/internal/*/schema.sql" "backend/internal/*/queries/*.sql" "backend/internal/*sqlc.yaml" "db/*.sql" \
		| $(DB_CHECK) vet'
