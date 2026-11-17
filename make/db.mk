# The local database, and the tests that need it. They run in check-all,
# because they start PostgreSQL in Docker.

CHECK_ALL_TARGETS += test-integration

DB := tools/scripts/db.sh

.PHONY: db-up db-apply db-plan db-down

db-up: ## Start the local PostgreSQL and bring every schema up to date
	@$(DB) up

db-apply: ## Apply the schema.sql of every context to the local database
	@$(DB) apply

db-plan: ## Print the DDL that db-apply would run
	@$(DB) plan

db-down: ## Stop the local PostgreSQL and delete its data
	@$(DB) down

# The tests behind the build tag `integration`: the queries, the constraints,
# and the privileges of the roles, against a real PostgreSQL (TPL-0009).
test-integration:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(GO_GLOBS) "*.sql" "compose.yaml")" ]; then $(DB) up > /dev/null && for m in $(GO_MODULES); do (cd "$$m" && go test -tags integration -race -shuffle=on ./...) || exit 1; done; fi'
