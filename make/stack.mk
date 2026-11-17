# The whole local system in Docker Compose (TPL-0017). The steps are in
# tools/scripts/stack.sh.

STACK := tools/scripts/stack.sh

.PHONY: up down

up: ## Start the local system: PostgreSQL with every schema, and every service
	@$(STACK) up

down: ## Stop the local system; the data of PostgreSQL stays (db-down deletes it)
	@$(STACK) down
