# Trivy: misconfiguration, vulnerable dependencies, and dependency licenses.
# Settings live in trivy.yaml. mise installs Trivy itself with a pinned version
# and checksum; see docs/adr/template/TPL-0024.

CHECK_TARGETS += scan-config
CHECK_ALL_TARGETS += scan-deps

export TRIVY_DISABLE_TELEMETRY := true
export TRIVY_SKIP_VERSION_CHECK := true

# Trivy reads public registries only. An empty Docker configuration keeps it
# from asking a credential helper, which can block without a terminal.
TRIVY_ENV := DOCKER_CONFIG=$(CURDIR)/.cache/trivy-docker-config
TRIVY_REPORT := --quiet --exit-code 1 --format template --template @tools/trivy/findings.tpl

# Dockerfile and Terraform misconfiguration, with the checks built into the
# pinned Trivy binary: no network, and the same result everywhere.
scan-config:
	@$(TASK) $@ 'trivy config $(TRIVY_REPORT) --skip-check-update --misconfig-scanners dockerfile,terraform .'

# Known vulnerabilities and forbidden licenses in every lockfile. Downloads the
# vulnerability database, so it runs in check-all.
scan-deps:
	@$(TASK) $@ 'mkdir -p .cache/trivy-docker-config && $(TRIVY_ENV) trivy fs $(TRIVY_REPORT) --scanners vuln,license .'
