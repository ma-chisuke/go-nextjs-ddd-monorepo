# SQL: squawk on every SQL file, with the rules that suit declarative schema
# files (.squawk.toml). The checks that need a database or the queries of a
# bounded context (sqlc vet, tbls lint, squawk on the DDL of a pgschema plan)
# arrive with the first context.

CHECK_TARGETS += lint-sql

lint-sql:
	@$(TASK) $@ '$(FILES) "*.sql" | xargs -r squawk --config=.squawk.toml --reporter=json \
		| jq -r -f tools/squawk/findings.jq'
