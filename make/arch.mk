# The architecture of the Go backend: the boundaries between bounded contexts
# (TPL-0005), the layers inside a context (TPL-0004), routes that come only
# from generated code (TPL-0007), SQL that stays in its own schema (TPL-0009),
# and the glossary of each context (TPL-0003). The rules are in the Go tool in
# tools/cmd/arch; they derive from the directory layout, so a new context
# needs no change here.

CHECK_TARGETS += lint-arch

# The files that the rules read, and the rules themselves. Generated Go files
# are on .lintignore, so a change to them alone does not start the check; the
# hand-written files next to them do.
ARCH_SOURCES := "backend/*.go" "backend/*.sql" "backend/go.mod" "docs/contexts/*" \
	"tools/cmd/arch/*" "tools/internal/arch/*"

# The check reads the whole backend: a rule about one file depends on others,
# such as the glossary on the code. So any file in scope starts a full run.
lint-arch:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(ARCH_SOURCES))" ]; then cd tools && go run ./cmd/arch -root "$(CURDIR)"; fi'
