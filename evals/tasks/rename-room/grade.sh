#!/usr/bin/env bash
# The criteria of the task rename-room (evals/run.sh sources this file and
# calls grade_task in the copy of the repository).
#
# The task adds an operation to the API of a supporting context. What it
# measures: an operation starts in the API document, from which the server,
# the clients of other contexts, and the types of the frontend are generated
# (TPL-0006, TPL-0007), and its use case reaches both stores: the queries of
# the real one, and the in-memory fake that the API tests use. `make check`
# holds the generated code (gen-check-api, gen-check-sqlc, gen-check-api-ts);
# the acceptance tests hold the two stores; the guide "Add an endpoint" asks
# for the API tests and for the table of operations in the README of the
# context.

api=backend/internal/facility/internal/adapter/http

# The tests of the task, in the package of the API tests, and away again.
put_acceptance() {
  cp "$TASK_DIR/acceptance_test.go" "$api/eval_acceptance_test.go"
  cp "$TASK_DIR/acceptance_integration_test.go" "$api/eval_acceptance_integration_test.go"
}

remove_acceptance() {
  rm -f "$api/eval_acceptance_test.go" "$api/eval_acceptance_integration_test.go"
}

# The API does what was asked, seen from outside, over the in-memory store.
renames_a_room() {
  put_acceptance
  local status=0
  go_test -count=1 -run '^TestEvalRenameRoom$' ./internal/facility/internal/adapter/http/ || status=$?
  remove_acceptance
  return "$status"
}

# The same over the real store, with the integration tests of Facility, in a
# database of the grader's own with the schema of the copy: the queries of
# the rename work against PostgreSQL.
real_store_renames() {
  put_acceptance
  local status=0
  with_database integration_tests internal/facility || status=$?
  remove_acceptance
  return "$status"
}

# The operation is in the API document, with an operationId (the guide, step
# 1); lint-contracts in `make check` fails on an operation without one, but
# not on an operation that is missing.
operation_in_the_document() {
  .venv/bin/python - << 'EOF'
import sys

import yaml

with open("contracts/http/facility.openapi.yaml", encoding="utf-8") as f:
    doc = yaml.safe_load(f)
operation = ((doc.get("paths") or {}).get("/rooms/{roomId}") or {}).get("patch") or {}
if not operation.get("operationId"):
    sys.exit("contracts/http/facility.openapi.yaml has no operation PATCH /rooms/{roomId} with an operationId")
EOF
}

# The operation has API tests next to the others (the guide, step 8).
api_tests_of_the_operation() {
  if ! changed_paths | grep -q "^$api/[^/]*_test\.go$"; then
    echo "no API test of Facility changed: the operation has no test in $api/"
    return 1
  fi
}

# The README of the context lists the operation, in both languages (the
# guide, step 9).
context_readme_names_the_operation() {
  if ! grep -q 'PATCH /rooms/' docs/contexts/facility/README.md || ! grep -q 'PATCH /rooms/' docs/contexts/facility/README.ja.md; then
    echo "docs/contexts/facility/README.md or its translation does not name PATCH /rooms/{roomId}"
    return 1
  fi
}

grade_task() {
  criterion renames-a-room renames_a_room
  criterion real-store-renames real_store_renames
  criterion operation-in-the-document operation_in_the_document
  criterion api-tests-of-the-operation api_tests_of_the_operation
  criterion context-readme-names-the-operation context_readme_names_the_operation
}
