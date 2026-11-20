#!/usr/bin/env bash
# The criteria of the task room-floor (evals/run.sh sources this file and
# calls grade_task in the copy of the repository).
#
# The task adds an optional property to a room. What it measures: the schema
# of the context, its queries, the code that sqlc and oapi-codegen generate,
# the stores, and the API document move together, and the change of the
# schema is one that a database with data can take (TPL-0010). `make check`
# holds the generated code and lint-sql; the acceptance tests hold the API
# over both stores; lint-sql-plan, which `make check-all` runs, holds the DDL
# that the change plans.
#
# The schema documents in docs/generated/db/ are not a criterion: they come
# from `make gen-db-docs`, which needs Docker, and the copy of a run has no
# Docker. lint-db-docs in `make check-all` reports them.

api=backend/internal/facility/internal/adapter/http
schema=backend/internal/facility/internal/adapter/postgres/schema.sql

# The tests of the task, in the package of the API tests, and away again.
put_acceptance() {
  cp "$TASK_DIR/acceptance_test.go" "$api/eval_acceptance_test.go"
  cp "$TASK_DIR/acceptance_integration_test.go" "$api/eval_acceptance_integration_test.go"
}

remove_acceptance() {
  rm -f "$api/eval_acceptance_test.go" "$api/eval_acceptance_integration_test.go"
}

# The API takes the floor and gives it back, seen from outside, over the
# in-memory store.
floor_in_the_api() {
  put_acceptance
  local status=0
  go_test -count=1 -run '^TestEvalRoomFloor$' ./internal/facility/internal/adapter/http/ || status=$?
  remove_acceptance
  return "$status"
}

# The same over the real store, with the integration tests of Facility, in a
# database of the grader's own with the schema of the copy: the floor goes
# into the table and comes back.
real_store_keeps_the_floor() {
  put_acceptance
  local status=0
  with_database integration_tests internal/facility || status=$?
  remove_acceptance
  return "$status"
}

# The table of the rooms has a column for the floor.
column_in_the_schema() {
  if ! grep -q -i -E '^[[:space:]]+"?[a-z_]*floor[a-z_]*"?[[:space:]]+[a-z]' "$schema"; then
    echo "$schema has no column for the floor"
    return 1
  fi
}

# The DDL that the change plans against a database that holds the schema of
# before passes the rules for a running database, without an allowance
# (the guide "Change a schema"; lint-sql-plan in `make check-all`).
schema_change_is_safe() {
  local out
  if ! out=$(CHECK_BASE=graded CHECK_BUDGET=0 make lint-sql-plan 2>&1); then
    printf '%s\n' "$out" | grep -v -e '^ok ' -e '^FAIL ' | tail -n 4
    return 1
  fi
}

# The API document describes the property, in what a registration takes and
# in what a room is.
property_in_the_document() {
  .venv/bin/python - << 'EOF'
import sys

import yaml

with open("contracts/http/facility.openapi.yaml", encoding="utf-8") as f:
    doc = yaml.safe_load(f)
schemas = (doc.get("components") or {}).get("schemas") or {}
for name in ("NewRoom", "Room"):
    if "floor" not in ((schemas.get(name) or {}).get("properties") or {}):
        sys.exit(f"contracts/http/facility.openapi.yaml: the schema {name} has no property floor")
EOF
}

grade_task() {
  criterion floor-in-the-api floor_in_the_api
  criterion real-store-keeps-the-floor real_store_keeps_the_floor
  criterion column-in-the-schema column_in_the_schema
  criterion schema-change-is-safe schema_change_is_safe
  criterion property-in-the-document property_in_the_document
}
