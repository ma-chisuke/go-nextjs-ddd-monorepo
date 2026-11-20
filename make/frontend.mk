# Frontend (Next.js): the types generated from the API documents, ESLint, the
# TypeScript compiler, knip, the unit tests, and the mutation tests of the
# unit tests. Prettier formats these files through make/repo.mk. Next.js 16
# removed `next lint` and `next build` no longer lints, so these targets are
# the only gate.
# Settings: frontend/eslint.config.mjs, frontend/tsconfig.json, frontend/knip.jsonc,
# frontend/stryker.config.mjs.

GEN_TARGETS += gen-api-ts
CHECK_TARGETS += gen-check-api-ts lint-frontend lint-frontend-types lint-frontend-unused test-frontend
CHECK_ALL_TARGETS += test-mutation-frontend

FRONTEND_BIN := $(CURDIR)/frontend/node_modules/.bin
FRONTEND_GEN := tools/scripts/gen-frontend.sh
FRONTEND_TS := "frontend/*.ts" "frontend/*.tsx" "frontend/*.mts" "frontend/*.cts"
FRONTEND_JS := "frontend/*.js" "frontend/*.jsx" "frontend/*.mjs" "frontend/*.cjs"
API_TS_SOURCES := "contracts/http/*" "frontend/src/lib/api/*" "frontend/package.json" "tools/scripts/gen-frontend.sh"

# No usage data leaves the machine.
export NEXT_TELEMETRY_DISABLED := 1

.PHONY: frontend-dev frontend-types

# The types of every API, from contracts/http/ with openapi-typescript
# (TPL-0006): frontend/src/lib/api/<context>.gen.ts.
gen-api-ts:
	@$(FRONTEND_GEN) api

# The generated types are what the generator writes now (TPL-0007, layer 2).
gen-check-api-ts:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(API_TS_SOURCES))" ]; then $(FRONTEND_GEN) api --check; fi'

# The types that Next.js writes for the routes and for imports that are not
# code (next-env.d.ts and .next/types). They are not in the repository, and
# the checks that read types need them, so those checks wait for this target.
# It writes them once, before the checks, and only when a frontend file is in
# scope.
frontend-types:
	@$(TASK) $@ 'if [ -n "$$($(FILES) frontend/)" ]; then cd frontend && $(FRONTEND_BIN)/next typegen > /dev/null; fi'

lint-frontend lint-frontend-types lint-frontend-unused: | frontend-types

# Every file in scope that ESLint understands, linted from frontend/.
lint-frontend:
	@$(TASK) $@ '$(FILES) $(FRONTEND_TS) $(FRONTEND_JS) | sed "s|^frontend/||" \
		| (cd frontend && xargs -r $(FRONTEND_BIN)/eslint --max-warnings 0 --no-warn-ignored --format json) \
		| jq -r --arg root "$(CURDIR)/" -f tools/eslint/findings.jq'

# The whole program, when a frontend file is in scope: the calls to the APIs
# against the generated types (TPL-0007, layer 1), and the links against the
# routes.
lint-frontend-types:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(FRONTEND_TS) frontend/tsconfig.json frontend/package.json)" ]; then \
		cd frontend && $(FRONTEND_BIN)/tsc --noEmit --pretty false \
		| sed -E "s|^([^ (][^(]*)\(([0-9]+),[0-9]+\): error (TS[0-9]+): |frontend/\1:\2: [\3] |"; fi'

# Files, exports, and dependencies that nothing uses, in the whole frontend.
lint-frontend-unused:
	@$(TASK) $@ 'if [ -n "$$($(FILES) frontend/)" ]; then \
		cd frontend && $(FRONTEND_BIN)/knip --no-progress --reporter json \
		| jq -r --arg dir frontend/ -f ../tools/knip/findings.jq; fi'

# The unit tests, with the test runner of Node, which runs TypeScript as it
# is: the modules that hold logic and import nothing of Next.js, such as
# src/lib/problem.ts. What a screen does is tested in the browser (make e2e).
test-frontend:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(FRONTEND_TS) frontend/package.json)" ]; then \
		cd frontend && node --test --test-reporter=dot "src/**/*.test.ts" 2>&1 \
		|| { node --test "src/**/*.test.ts" 2>&1 | grep -vE "^(✔|ℹ)"; exit 1; }; fi'

# Mutation testing of the pure logic (TPL-0023): StrykerJS changes the code of
# each module of src/ that has a unit test next to it, in small ways, and runs
# the unit tests against each change. A change that every test passes fails
# the check, unless a reviewed line of equivalent-mutants.allow shows that no
# test can tell it apart. The modules follow from the tests, so a new module
# with a test needs no edit here. Run some modules with
# `make test-mutation-frontend MUTATION_MODULES=frontend/src/lib/input.ts`, and
# one mutant with `make mutant-frontend ID=<mutant>`.
# See docs/guides/mutation-tests.md.
FRONTEND_UNIT_TESTS := $(shell find frontend/src -name '*.test.ts' 2> /dev/null)
MUTATION_FRONTEND_SOURCES := $(foreach t,$(FRONTEND_UNIT_TESTS),"$(t)" "$(t:.test.ts=.ts)") \
	"frontend/stryker.config.mjs" "frontend/package.json" "equivalent-mutants.allow" "tools/stryker"
MUTATION_MODULES ?=

.PHONY: mutant-frontend

test-mutation-frontend:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(MUTATION_FRONTEND_SOURCES))" ]; then \
		tools/scripts/mutation.sh frontend $(MUTATION_MODULES); fi'

mutant-frontend: ## Run one mutant that test-mutation-frontend found: make mutant-frontend ID=<mutant>
	@tools/scripts/mutation.sh frontend-mutant "$(ID)"

# The development server of Next.js, against the APIs of the local system
# (`make up`), on http://127.0.0.1:3000.
frontend-dev: ## Run the frontend from source against the local system
	@cd frontend && FACILITY_API_URL=$${FACILITY_API_URL:-http://127.0.0.1:$${FACILITY_PORT:-18081}} \
		RESERVATION_API_URL=$${RESERVATION_API_URL:-http://127.0.0.1:$${RESERVATION_PORT:-18082}} \
		$(FRONTEND_BIN)/next dev --hostname 127.0.0.1
