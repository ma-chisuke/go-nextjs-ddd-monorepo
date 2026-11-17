# Checks that apply to every file, and the formats without a home of their own:
# YAML, JSON, TOML, shell, and Dockerfiles.

FMT_TARGETS += fmt-prettier fmt-toml fmt-shell
CHECK_TARGETS += fmt-check-prettier fmt-check-toml fmt-check-shell \
	lint-yaml lint-toml lint-schema lint-shell lint-dockerfile \
	lint-typos lint-editorconfig lint-secrets
CHECK_ALL_TARGETS += lint-schema-remote

PRETTIER_GLOBS := "*.yaml" "*.yml" "*.json" "*.jsonc" "*.json5" "*.js" "*.jsx" "*.mjs" "*.cjs" "*.ts" "*.tsx" "*.css"
SHELL_GLOBS := "*.sh"
DOCKERFILE_GLOBS := "*Dockerfile" "*Dockerfile.*" "*.Dockerfile"

fmt-prettier:
	@$(FILES) $(PRETTIER_GLOBS) | xargs -r prettier --write --log-level warn

fmt-check-prettier:
	@$(TASK) $@ '$(FILES) $(PRETTIER_GLOBS) | xargs -r prettier --check --log-level warn'

fmt-toml:
	@$(FILES) "*.toml" | RUST_LOG=warn xargs -r taplo format

fmt-check-toml:
	@$(TASK) $@ '$(FILES) "*.toml" | RUST_LOG=warn xargs -r taplo format --check'

fmt-shell:
	@$(FILES) $(SHELL_GLOBS) | xargs -r shfmt --write

fmt-check-shell:
	@$(TASK) $@ '$(FILES) $(SHELL_GLOBS) | xargs -r shfmt --diff'

lint-yaml:
	@$(TASK) $@ '$(FILES) "*.yaml" "*.yml" | xargs -r yamllint --strict --format parsable'

lint-toml:
	@$(TASK) $@ '$(FILES) "*.toml" | RUST_LOG=warn xargs -r taplo lint --no-schema'

# Configuration files validated against the schemas bundled with
# check-jsonschema, so this check needs no network.
lint-schema:
	@$(TASK) $@ '\
		$(FILES) "renovate.json" "renovate.json5" ".github/renovate.json" ".github/renovate.json5" | xargs -r check-jsonschema --builtin-schema vendor.renovate; \
		$(FILES) "compose.yaml" "compose.*.yaml" | xargs -r check-jsonschema --builtin-schema vendor.compose-spec'

# Configuration files whose schema is fetched from its publisher.
lint-schema-remote:
	@$(TASK) $@ '\
		$(FILES) "lefthook.yml" | xargs -r check-jsonschema --schemafile https://raw.githubusercontent.com/evilmartians/lefthook/v2.2.2/schema.json; \
		$(FILES) "mise.toml" | xargs -r check-jsonschema --schemafile https://mise.jdx.dev/schema/mise.json; \
		$(FILES) ".devcontainer/devcontainer.json" | xargs -r check-jsonschema --schemafile https://raw.githubusercontent.com/devcontainers/spec/c95ffeed1d059abfe9ffbe79762dc2fa4e7c2421/schemas/devContainer.schema.json'

lint-shell:
	@$(TASK) $@ '$(FILES) $(SHELL_GLOBS) | xargs -r shellcheck --format=gcc --external-sources'

lint-dockerfile:
	@$(TASK) $@ '$(FILES) $(DOCKERFILE_GLOBS) | xargs -r hadolint --no-color'

lint-typos:
	@$(TASK) $@ '$(FILES) | xargs -r typos --format brief --force-exclude'

lint-editorconfig:
	@$(TASK) $@ '$(FILES) | xargs -r editorconfig-checker -no-color -format gcc'

# Secrets: the staged changes before a commit, otherwise the working tree.
# The secret itself is never printed.
GITLEAKS_REPORT := --no-banner --redact --log-level error --report-format template --report-template tools/gitleaks/findings.tpl --report-path -
lint-secrets:
ifeq ($(CHECK_SCOPE),staged)
	@$(TASK) $@ 'gitleaks git --staged $(GITLEAKS_REPORT)'
else
	@$(TASK) $@ 'gitleaks dir $(GITLEAKS_REPORT) .'
endif
