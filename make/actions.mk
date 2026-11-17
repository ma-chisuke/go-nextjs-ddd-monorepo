# GitHub Actions workflows: syntax, security, policy, and pinning.

CHECK_TARGETS += lint-actions lint-actions-security lint-actions-policy lint-actions-pins

WORKFLOW_GLOBS := ".github/workflows/*.yml" ".github/workflows/*.yaml"
# Run a command when workflow files are in scope.
if-workflows = if [ -n "$$($(FILES) $(WORKFLOW_GLOBS))" ]; then $(1); fi

lint-actions:
	@$(TASK) $@ '$(FILES) $(WORKFLOW_GLOBS) | xargs -r actionlint -oneline'

lint-actions-security:
	@$(TASK) $@ '$(call if-workflows,zizmor --offline --no-progress --format plain .github/)'

# Least privilege and a timeout on every job.
lint-actions-policy:
	@$(TASK) $@ '$(call if-workflows,ghalint run)'

# Every action is pinned to a full commit SHA.
lint-actions-pins:
	@$(TASK) $@ '$(call if-workflows,pinact run --check)'
