# Go: every directory that holds a go.mod is checked as its own module.
# golangci-lint runs the formatters (gofmt, goimports) and the linters,
# including go vet, so each concern has one tool. See .golangci.yml.

SETUP_TARGETS += setup-go
FMT_TARGETS += fmt-go
CHECK_TARGETS += fmt-check-go lint-go test-go
CHECK_ALL_TARGETS += vuln-go test-mutation

GO_MODULES := $(patsubst %/go.mod,%,$(wildcard */go.mod))
GO_GLOBS := "*.go" "*go.mod" "*go.sum" ".golangci.yml"
GOLANGCI := golangci-lint --config $(CURDIR)/.golangci.yml

# Run a command in every Go module that has Go files, when Go files are in scope.
go-each = if [ -n "$$($(FILES) $(GO_GLOBS))" ]; then for m in $(GO_MODULES); do if [ -n "$$(find "$$m" -name "*.go" -print -quit)" ]; then (cd "$$m" && $(1)); fi; done; fi

# Download the modules and compile once, so that `make check` needs no network
# and its first run is as fast as the next ones.
setup-go:
	@for m in $(GO_MODULES); do (cd "$$m" && go mod download && go build ./... && go test -race -run "^$$" ./... > /dev/null); done

fmt-go:
	@$(call go-each,$(GOLANGCI) fmt ./...)

fmt-check-go:
	@$(TASK) $@ '$(call go-each,$(GOLANGCI) fmt --diff ./...)'

lint-go:
	@$(TASK) $@ '$(call go-each,$(GOLANGCI) run ./...)'

test-go:
	@$(TASK) $@ '$(call go-each,go test -race -shuffle=on ./...)'

# Known vulnerabilities in code that the module actually calls.
vuln-go:
	@$(TASK) $@ '$(call go-each,go tool -modfile=$(CURDIR)/tools/go.mod govulncheck ./...)'

# Mutation testing: gomutants changes the code of these packages in small ways
# and runs the tests of each package against each change. A change that every
# test passes fails the check, unless a reviewed line of
# equivalent-mutants.allow shows that no test can tell it apart. The list
# holds the domain and the use cases of each core context (TPL-0003): add
# those of a new core context. Run one package with
# `make test-mutation MUTATION_PACKAGES=<directory>`, and one mutant with
# `make mutant ID=<mutant>`. See docs/guides/mutation-tests.md.
MUTATION_PACKAGES := \
	backend/internal/reservation/internal/domain \
	backend/internal/reservation/internal/app
MUTATION := tools/scripts/mutation.sh
MUTATION_SOURCES := $(foreach p,$(MUTATION_PACKAGES),"$(p)/*.go") "backend/go.mod" "equivalent-mutants.allow" "tools/gomutants"

.PHONY: mutant

test-mutation:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(MUTATION_SOURCES))" ]; then $(MUTATION) go $(MUTATION_PACKAGES); fi'

mutant: ## Run one mutant that test-mutation found: make mutant ID=<mutant>
	@$(MUTATION) go-mutant "$(ID)"

# Coverage: one profile per module in coverage/go/, merged into coverage/go.out,
# which .octocov.yml reads. `make coverage` runs this target (make/coverage.mk).
COVERAGE_TARGETS += coverage-go
.PHONY: coverage-go

coverage-go:
	@rm -rf coverage/go coverage/go.out && mkdir -p coverage/go
	@$(call go-each,go test -race -shuffle=on -covermode=atomic -coverprofile="$(CURDIR)/coverage/go/$$m.out" ./...)
	@if [ -n "$$(find coverage/go -name '*.out' -print -quit)" ]; then \
		{ echo "mode: atomic"; for f in coverage/go/*.out; do tail -n +2 "$$f"; done; } > coverage/go.out; \
	fi
