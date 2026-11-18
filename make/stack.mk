# The whole local system in Docker Compose (TPL-0017), and the smoke test that
# runs one scenario across the services over real HTTP. The steps are in
# tools/scripts/stack.sh.

CHECK_ALL_TARGETS += smoke e2e

STACK := tools/scripts/stack.sh

# What goes into the images or into the system: a change to any of these
# starts the smoke test and the scan of the images (make/security.mk).
STACK_SOURCES := "backend" "frontend" "pnpm-lock.yaml" "contracts" "db" "compose.yaml" "infra/local" "infra/modules" "tools/scripts/aws.sh"

.PHONY: up down

up: ## Start the local system: PostgreSQL with every schema, and every service
	@$(STACK) up

down: ## Stop the local system; the data of PostgreSQL stays (db-down deletes it)
	@$(STACK) down

# One scenario across Facility and Reservation over real HTTP, with every
# response checked against the API documents, and the mail that the events of
# the scenario cause (backend/test/smoke). It starts the services on the
# PostgreSQL and the AWS Emulator that db-server and aws-server started
# (make/db.mk, make/aws.mk), stops the services that it started, and leaves
# those two running.
smoke e2e: | db-server
smoke: ## Start the local system and run the smoke test against it
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(STACK_SOURCES))" ]; then $(STACK) smoke; fi'

# The scenarios of a person in a browser, from a screen to the mail, with
# Playwright (frontend/e2e). It starts the system as the smoke test does, and
# waits for the smoke test when both run (tools/scripts/stack.sh).
e2e: ## Start the local system and run the end-to-end test in a browser
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(STACK_SOURCES))" ]; then $(STACK) e2e; fi'
