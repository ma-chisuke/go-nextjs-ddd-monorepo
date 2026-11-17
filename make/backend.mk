# The backend: the code generated from the API documents and from the SQL of
# each bounded context. The Go checks themselves are in make/go.mk, which
# treats backend/ like every other Go module.

GEN_TARGETS += gen-api gen-sqlc
CHECK_TARGETS += gen-check-api gen-check-sqlc

BACKEND_GEN := tools/scripts/gen-backend.sh
API_SOURCES := "contracts/http/*" "*oapi-codegen.yaml" "backend/*.gen.go" "tools/go.mod"
SQLC_SOURCES := "*sqlc.yaml" "*schema.sql" "backend/*/queries/*.sql" "backend/*.gen.go"

# Servers and clients, from contracts/http/ with oapi-codegen (TPL-0006).
gen-api:
	@$(BACKEND_GEN) api

# Typed queries, from each context's schema.sql and queries/ with sqlc (TPL-0011).
gen-sqlc:
	@$(BACKEND_GEN) sqlc

# The generated code is what the generators write now (TPL-0007, layer 2).
gen-check-api:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(API_SOURCES))" ]; then $(BACKEND_GEN) api --check; fi'

gen-check-sqlc:
	@$(TASK) $@ 'if [ -n "$$($(FILES) $(SQLC_SOURCES))" ]; then $(BACKEND_GEN) sqlc --check; fi'
