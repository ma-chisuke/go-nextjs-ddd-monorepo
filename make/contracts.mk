# API contracts: Redocly CLI lints the OpenAPI 3.1 documents in contracts/http
# and the AsyncAPI 3.0 documents in contracts/events. Settings: redocly.yaml.
# oasdiff finds the breaking changes of each OpenAPI document.

CHECK_TARGETS += lint-contracts lint-contracts-breaking

CONTRACT_GLOBS := "contracts/*.openapi.yaml" "contracts/*.asyncapi.yaml"

# No telemetry and no update check, so the check never leaves the machine.
export REDOCLY_TELEMETRY := off
export REDOCLY_SUPPRESS_NOTIFICATIONS := true

# Redocly writes its progress to stderr. The report goes through jq, and the
# rest of stderr (a broken document or configuration) is shown on failure.
REDOCLY_STDERR := .cache/redocly.stderr
REDOCLY_PROGRESS := ^(validating .*|.*: validated in .*|❌ Validation failed.*|run .redocly lint --generate-ignore-file.*|Woohoo!.*|[[:space:]]*)\$$

lint-contracts:
	@$(TASK) $@ 'mkdir -p .cache && $(FILES) $(CONTRACT_GLOBS) \
		| xargs -r redocly lint --config=redocly.yaml --lint-config=error --format=json --max-problems=1000 2> $(REDOCLY_STDERR) \
		| jq -r -f tools/redocly/findings.jq || { grep -Ev "$(REDOCLY_PROGRESS)" $(REDOCLY_STDERR); exit 1; }'

# Each OpenAPI document against the same file at the base commit, the merge
# base with origin/main or with CHECK_BASE (TPL-0007, layer 6). Offline: the
# base comes from Git. See tools/scripts/contracts-breaking.sh.
lint-contracts-breaking:
	@$(TASK) $@ '$(FILES) "contracts/http/*.openapi.yaml" | tools/scripts/contracts-breaking.sh'

# Requests that Schemathesis generates from each OpenAPI document, sent to the
# API of its context, built from this checkout and run on a database of its
# own on the local PostgreSQL (TPL-0007, layer 5). It needs PostgreSQL and
# takes minutes, so it runs in check-all, when a file in scope can change what
# the APIs answer: a document, Go code, SQL, or the check itself. See
# tools/scripts/api-fuzz.sh and tools/schemathesis/.
CHECK_ALL_TARGETS += test-api-fuzz

test-api-fuzz: | db-server
test-api-fuzz:
	@$(TASK) $@ 'if [ -n "$$($(FILES) "contracts/http/*" $(GO_GLOBS) "*.sql" "tools/schemathesis/*" "tools/scripts/api-fuzz.sh")" ]; then tools/scripts/api-fuzz.sh; fi'

# The JSON Schema of each event, contracts/events/<context>/<Name>.v<N>.json
# (TPL-0013), with the Go tool in tools/cmd/events. The rules are in
# docs/guides/event-changes.md.

CHECK_TARGETS += lint-events lint-events-breaking

# The files that the checks read, and the checks themselves.
EVENT_SOURCES := "contracts/events/*" "tools/cmd/events/*" "tools/internal/events/*"

# Each schema is well formed, and a message of the AsyncAPI document of its
# context refers to it. A schema depends on that document, so any file in
# scope starts a run over every schema, whatever the scope.
lint-events:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(EVENT_SOURCES))" ]; then \
		env -u CHECK_SCOPE -u CHECK_FILES $(FILES) "contracts/events/*.json" | (cd tools && go run ./cmd/events -root "$(CURDIR)" lint); fi'

# Each schema in scope against the same file at the base commit, as
# lint-contracts-breaking does, and every schema of the base must still exist.
# A deleted file is never in scope, so a change that only deletes a schema is
# caught by a run over every file, such as `make check` without a scope.
# See tools/scripts/events-breaking.sh.
lint-events-breaking:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(EVENT_SOURCES))" ]; then \
		$(FILES) "contracts/events/*.json" | tools/scripts/events-breaking.sh; fi'
