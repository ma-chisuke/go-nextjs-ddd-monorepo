# The local database, and the tests and checks that need it. They run in
# check-all, because they start PostgreSQL in Docker. The checks of SQL are
# in make/sql.mk.

CHECK_ALL_TARGETS += test-integration

DB := tools/scripts/db.sh

.PHONY: db-up db-apply db-plan db-down db-server

db-up: ## Start the local PostgreSQL and bring every schema up to date
	@$(DB) up

db-apply: ## Apply the schema.sql of every context to the local database
	@$(DB) apply

db-plan: ## Print the DDL that db-apply would run
	@$(DB) plan

db-down: ## Stop the local PostgreSQL and delete its data
	@$(DB) down

# Every target that needs the server waits for this one, so the server
# starts once, before them, and never twice at the same time. It changes no
# database, and it leaves a running server alone, so a copy of the repository
# can run the checks too (tools/selftest/selftest.sh).
db-server:
	@$(TASK) $@ '$(DB) server'

test-integration selftest: | db-server

# The tests behind the build tag `integration`: the queries, the constraints,
# and the privileges of the roles, against a real PostgreSQL (TPL-0009).
test-integration:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(GO_GLOBS) "*.sql" "compose.yaml")" ]; then $(DB) init > /dev/null && for m in $(GO_MODULES); do (cd "$$m" && go test -tags integration -race -shuffle=on ./...) || exit 1; done; fi'
