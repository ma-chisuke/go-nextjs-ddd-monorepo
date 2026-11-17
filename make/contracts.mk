# API contracts: Redocly CLI lints the OpenAPI 3.1 documents in contracts/http
# and the AsyncAPI 3.0 documents in contracts/events. Settings: redocly.yaml.

CHECK_TARGETS += lint-contracts

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
