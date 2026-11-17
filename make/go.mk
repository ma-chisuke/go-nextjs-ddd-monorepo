# Go: every directory that holds a go.mod is checked as its own module.
# golangci-lint runs the formatters (gofmt, goimports) and the linters,
# including go vet, so each concern has one tool. See .golangci.yml.

FMT_TARGETS += fmt-go
CHECK_TARGETS += fmt-check-go lint-go test-go
CHECK_ALL_TARGETS += vuln-go

GO_MODULES := $(patsubst %/go.mod,%,$(wildcard */go.mod))
GO_GLOBS := "*.go" "*go.mod" "*go.sum" ".golangci.yml"
GOLANGCI := golangci-lint --config $(CURDIR)/.golangci.yml

# Run a command in every Go module that has Go files, when Go files are in scope.
go-each = if [ -n "$$($(FILES) $(GO_GLOBS))" ]; then for m in $(GO_MODULES); do if [ -n "$$(find "$$m" -name "*.go" -print -quit)" ]; then (cd "$$m" && $(1)); fi; done; fi

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
