# Trivy: misconfiguration, vulnerable dependencies, dependency licenses, and
# the images of the services with their SBOMs. Settings live in trivy.yaml.
# mise installs Trivy itself with a pinned version and checksum; see
# docs/adr/template/TPL-0024. Also the pins of base images to digests.

CHECK_TARGETS += scan-config lint-image-pins
CHECK_ALL_TARGETS += scan-deps scan-images

export TRIVY_DISABLE_TELEMETRY := true
export TRIVY_SKIP_VERSION_CHECK := true

# Trivy reads public registries only. An empty Docker configuration keeps it
# from asking a credential helper, which can block without a terminal.
TRIVY_ENV := DOCKER_CONFIG=$(CURDIR)/.cache/trivy-docker-config
TRIVY_REPORT := --quiet --exit-code 1 --format template --template @tools/trivy/findings.tpl
# Directories that hold no source of this repository.
TRIVY_SKIP := --skip-dirs .venv --skip-dirs .cache --skip-dirs .claude/worktrees --skip-dirs tmp

# Dockerfile and Terraform misconfiguration, with the checks built into the
# pinned Trivy binary: no network, and the same result everywhere. The cache of
# its own keeps out a check bundle that an earlier `trivy` run downloaded.
# .trivyignore.yaml lists the findings that review accepted, each for the
# files that it names.
scan-config:
	@$(TASK) $@ 'trivy config $(TRIVY_REPORT) --cache-dir $(CURDIR)/.cache/trivy-config --skip-check-update --misconfig-scanners dockerfile,terraform --ignorefile .trivyignore.yaml $(TRIVY_SKIP) --skip-dirs node_modules --skip-dirs "**/node_modules" .'

# Dependencies in every lockfile. Downloads the vulnerability database, so it
# runs in check-all.
#   Vulnerabilities: every dependency, including development tools, because
#     they run on laptops and in CI.
#   Licenses: only the licenses that trivy.yaml forbids fail (Trivy rates them
#     CRITICAL), and only in what ships with the product, so development dependencies
#     and the tools/ module are left out. Trivy reads the license of an
#     npm package from node_modules and that of a Go module from the module
#     cache, so node_modules is not skipped and the Go modules are downloaded.
scan-deps:
	@$(TASK) $@ 'mkdir -p .cache/trivy-docker-config; \
		for m in $(GO_MODULES); do (cd "$$m" && go mod download); done; \
		$(TRIVY_ENV) trivy fs $(TRIVY_REPORT) --scanners vuln --include-dev-deps $(TRIVY_SKIP) .; \
		$(TRIVY_ENV) trivy fs $(TRIVY_REPORT) --scanners license --severity CRITICAL $(TRIVY_SKIP) --skip-dirs tools .'

# Base images pinned by digest (TPL-0024), so a moved tag changes no image.
# Neither hadolint nor Trivy has this rule.
lint-image-pins:
	@$(TASK) $@ '$(FILES) $(DOCKERFILE_GLOBS) | xargs -r tools/scripts/image-pins.sh'

# Trivy reads the images from the Docker daemon of the current context of the
# Docker CLI, whose configuration TRIVY_ENV hides, so the address of the daemon
# is passed on. The images get a cache of their own: scan-images runs next to
# scan-deps in check-all, and two Trivy processes must not write one cache at
# the same time.
TRIVY_DOCKER := DOCKER_HOST="$${DOCKER_HOST:-$$(docker context inspect --format "{{.Endpoints.docker.Host}}")}"
TRIVY_IMAGE := $(TRIVY_ENV) $(TRIVY_DOCKER) trivy image --image-src docker --cache-dir $(CURDIR)/.cache/trivy-images

# Known vulnerabilities in the image of every service that compose.yaml builds
# (backend/Dockerfile): in the packages of the base image and in the modules
# of the binary, with the severities of trivy.yaml. It builds the images and
# downloads the vulnerability database, so it runs in check-all, when the
# sources of the images are in scope (STACK_SOURCES in make/stack.mk).
scan-images:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(STACK_SOURCES))" ]; then \
		mkdir -p .cache/trivy-docker-config && $(STACK) build && code=0 && \
		while read -r service image; do \
			$(TRIVY_IMAGE) $(TRIVY_REPORT) --scanners vuln "$$image" < /dev/null || code=1; \
		done < <($(STACK) images) && exit $$code; fi'

# One CycloneDX SBOM per image, dist/sbom/<service>.cdx.json (TPL-0024): the
# packages of the base image and the modules of the binary. It lists no
# vulnerabilities, so it needs no database; a scan of the SBOM later
# (`trivy sbom`) reports what is known then. The workflow that publishes the
# images attests these files with GitHub artifact attestations.
.PHONY: sbom
sbom: ## Write an SBOM of the image of every service to dist/sbom/
	@mkdir -p dist/sbom .cache/trivy-docker-config
	@$(STACK) build
	@$(STACK) images | while read -r service image; do \
		$(TRIVY_IMAGE) --quiet --format cyclonedx --output "dist/sbom/$$service.cdx.json" "$$image" < /dev/null || exit 1; \
		echo "sbom: dist/sbom/$$service.cdx.json ($$image)"; \
	done
