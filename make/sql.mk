# SQL: squawk on every SQL file, with the rules that suit declarative schema
# files (.squawk.toml). In check-all, because they need PostgreSQL: squawk on
# the DDL of each schema change.

CHECK_TARGETS += lint-sql
CHECK_ALL_TARGETS += lint-sql-plan

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
