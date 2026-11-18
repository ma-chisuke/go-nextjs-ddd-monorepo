# The whole local system in Docker Compose (TPL-0017), and the smoke test that
# runs one scenario across the services over real HTTP. The steps are in
# tools/scripts/stack.sh.

CHECK_ALL_TARGETS += smoke

STACK := tools/scripts/stack.sh

# What goes into the images or into the system: a change to any of these
# starts the smoke test and the scan of the images (make/security.mk).
STACK_SOURCES := "backend" "contracts" "db" "compose.yaml" "infra/local" "infra/modules" "tools/scripts/aws.sh"

.PHONY: up down

up: ## Start the local system: PostgreSQL with every schema, and every service
	@$(STACK) up

down: ## Stop the local system; the data of PostgreSQL stays (db-down deletes it)
	@$(STACK) down

# One scenario across Facility and Reservation over real HTTP, with every
# response checked against the API documents (backend/test/smoke). It starts
# the services on the PostgreSQL that db-server started (make/db.mk), stops
# the services that it started, and leaves PostgreSQL running.
smoke: | db-server
smoke: ## Start the local system and run the smoke test against it
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(STACK_SOURCES))" ]; then $(STACK) smoke; fi'
