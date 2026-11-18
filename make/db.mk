# The local database, the tests and checks that need it, and the schema
# documents written from it. They run in check-all, because they start
# PostgreSQL in Docker. The checks of SQL are in make/sql.mk.

CHECK_ALL_TARGETS += test-integration lint-db-docs

DB := tools/scripts/db.sh

.PHONY: db-up db-apply db-plan db-down db-server gen-db-docs

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

test-integration selftest gen-db-docs: | db-server

# The tests behind the build tag `integration`: the queries, the constraints,
# and the privileges of the roles, against a real PostgreSQL (TPL-0009), and
# the adapters for EventBridge and SQS against AWS Emulator (TPL-0017), which
# make/aws.mk starts.
#
# The tests have a database of their own on the local server, which db.sh
# creates. The local system (`make up`, `make smoke`) uses another one, so a
# test never meets the rows of the system, and the relay of the system never
# takes an event that a test put into an outbox.
TEST_DATABASE := app_test

test-integration:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(GO_GLOBS) "*.sql" "compose.yaml" $(AWS_SOURCES))" ]; then export PGDATABASE=$(TEST_DATABASE) && $(DB) init > /dev/null && eval "$$($(AWS) env)" && for m in $(GO_MODULES); do (cd "$$m" && go test -tags integration -race -shuffle=on ./...) || exit 1; done; fi'

# The schema documents of each context in docs/generated/db/<context>/,
# written by tbls from a scratch database. Not part of `make gen`, which needs
# no database: run this after a change to a schema.sql (lint-db-docs says so).
gen-db-docs: ## Write the schema documents of every context (needs Docker)
	@$(DB_CHECK) docs

# Every table and column has a comment, and the schema documents are what
# gen-db-docs writes now (TPL-0023).
lint-db-docs: | db-server
	@$(TASK) $@ '$(FILES) "backend/internal/*/schema.sql" "db/*.sql" ".tbls.yml" | $(DB_CHECK) docs-lint'
