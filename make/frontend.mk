# Frontend (Next.js): ESLint, the TypeScript compiler, and knip. Prettier
# formats these files through make/repo.mk. Next.js 16 removed `next lint` and
# `next build` no longer lints, so these targets are the only gate.
# Settings: frontend/eslint.config.mjs, frontend/tsconfig.json, frontend/knip.jsonc.

CHECK_TARGETS += lint-frontend lint-frontend-types lint-frontend-unused

FRONTEND_BIN := $(CURDIR)/frontend/node_modules/.bin
FRONTEND_TS := "frontend/*.ts" "frontend/*.tsx" "frontend/*.mts" "frontend/*.cts"
FRONTEND_JS := "frontend/*.js" "frontend/*.jsx" "frontend/*.mjs" "frontend/*.cjs"

# Every file in scope that ESLint understands, linted from frontend/.
lint-frontend:
	@$(TASK) $@ '$(FILES) $(FRONTEND_TS) $(FRONTEND_JS) | sed "s|^frontend/||" \
		| (cd frontend && xargs -r $(FRONTEND_BIN)/eslint --max-warnings 0 --no-warn-ignored --format json) \
		| jq -r --arg root "$(CURDIR)/" -f tools/eslint/findings.jq'

# The whole program, when a frontend file is in scope. tsc fails on a project
# without TypeScript files, so it runs only once the first one exists.
lint-frontend-types:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(FRONTEND_TS) frontend/tsconfig.json frontend/package.json)" ] \
		&& [ -n "$$(CHECK_SCOPE=all CHECK_FILES= $(FILES) $(FRONTEND_TS))" ]; then \
		cd frontend && $(FRONTEND_BIN)/tsc --noEmit --pretty false \
		| sed -E "s|^([^ (][^(]*)\(([0-9]+),[0-9]+\): error (TS[0-9]+): |frontend/\1:\2: [\3] |"; fi'

# Files, exports, and dependencies that nothing uses, in the whole frontend.
lint-frontend-unused:
	@$(TASK) $@ 'if [ -n "$$($(FILES) frontend/)" ]; then \
		cd frontend && $(FRONTEND_BIN)/knip --no-progress --reporter json \
		| jq -r --arg dir frontend/ -f ../tools/knip/findings.jq; fi'
