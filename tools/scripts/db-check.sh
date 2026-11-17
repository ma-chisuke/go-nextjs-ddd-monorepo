#!/usr/bin/env bash
# The checks that need PostgreSQL. Each run works in a
# scratch database of the local server, which it creates and drops, so it
# never changes the schemas in the database app (TPL-0009).
#
# Usage: db-check.sh plan        lint the DDL of each schema change (TPL-0010)
#
# plan reads the paths in scope on stdin, one per line
# (tools/scripts/files.sh). A path under backend/internal/<context>/ selects
# that context; any other path, such as db/bootstrap.sql, selects every
# context. With no path, nothing runs.
#
# plan compares with the base commit (tools/scripts/merge-base.sh): pgschema
# applies the schema.sql of the base to the scratch database (none when the
# file is new), plans the change to the schema.sql of the working tree, and
# squawk lints that plan (.squawk.plan.toml). The plan is kept in
# .cache/sql-plan/<context>.sql, where the findings point. Without a base,
# nothing is compared.
#
# The scratch database gets what every database of the repository gets
# first, db/bootstrap.sql and db/local.sql, as `db.sh up` gives app. Roles
# belong to the whole server, and those files only create what is missing,
# so this changes nothing that app relies on. pgschema then applies the
# schema of each context as its owner role, as `db.sh apply` does.
#
# The server must run (`db.sh server`). The connection follows db.sh: PGHOST
# (127.0.0.1), POSTGRES_PORT (55432), DB_OWNER_PASSWORD_<CONTEXT>, and
# PGSCHEMA_PLAN_USER and PGSCHEMA_PLAN_PASSWORD.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

host="${PGHOST:-127.0.0.1}"
port="${POSTGRES_PORT:-55432}"
sslmode="${PGSSLMODE:-disable}"
kind="${1:-}"

tmp=$(mktemp -d)
scratch=""
cleanup() {
  if [ -n "$scratch" ]; then
    psql_in postgres --command "DROP DATABASE IF EXISTS $scratch WITH (FORCE)" > /dev/null 2>&1 || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

contexts() {
  local file
  for file in backend/internal/*/internal/adapter/postgres/schema.sql; do
    [ -f "$file" ] || continue
    file=${file#backend/internal/}
    printf '%s\n' "${file%%/*}"
  done
}

dir_of() {
  printf 'backend/internal/%s/internal/adapter/postgres\n' "$1"
}

# The contexts that the paths on stdin select, one per line.
selected() {
  local path wanted='' all=false context
  while IFS= read -r path; do
    case "$path" in
      backend/internal/*/*)
        path=${path#backend/internal/}
        wanted="$wanted${path%%/*}"$'\n'
        ;;
      ?*) all=true ;;
    esac
  done
  for context in $(contexts); do
    if [ "$all" = true ] || printf '%s' "$wanted" | grep -Fxq "$context"; then
      printf '%s\n' "$context"
    fi
  done
}

owner_password() {
  local variable
  variable="DB_OWNER_PASSWORD_$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')"
  printf '%s' "${!variable:-${1}_owner}"
}

psql_in() {
  local database="$1"
  shift
  docker compose exec -T postgres psql --no-psqlrc --quiet --set ON_ERROR_STOP=1 \
    --username postgres --dbname "$database" "$@"
}

# Two runs of these files, in any two databases, can change the same role at
# once, and PostgreSQL then fails one of them; they can run again, so a
# failed run is tried twice more (as in db.sh).
create_scratch() {
  local file tries
  scratch="check_${kind//-/_}_$$_${RANDOM}"
  psql_in postgres --command "CREATE DATABASE $scratch" > /dev/null
  for file in db/bootstrap.sql db/local.sql; do
    tries=0
    until psql_in "$scratch" --file - < "$file" > "$tmp/psql.log" 2>&1; do
      tries=$((tries + 1))
      if [ "$tries" -ge 3 ]; then
        grep -v 'NOTICE:' "$tmp/psql.log"
        return 1
      fi
      sleep 1
    done
  done
}

# pgschema on the scratch database, as the owner role of the context, which
# is also where it validates the desired state.
pgschema_in() {
  local command="$1" context="$2" file="$3"
  shift 3
  PGPASSWORD="$(owner_password "$context")" pgschema "$command" \
    --host "$host" --port "$port" --db "$scratch" --sslmode "$sslmode" \
    --user "${context}_owner" --schema "$context" --file "$file" \
    --plan-host "$host" --plan-port "$port" --plan-db "$scratch" --plan-sslmode "$sslmode" \
    --plan-user "${PGSCHEMA_PLAN_USER:-postgres}" --plan-password "${PGSCHEMA_PLAN_PASSWORD:-postgres}" \
    --no-color "$@"
}

# Apply a schema file to the scratch database. On failure, say which file
# and show what pgschema said.
apply_schema() {
  local context="$1" file="$2" label="${3:-$2}"
  if ! pgschema_in apply "$context" "$file" --auto-approve > "$tmp/apply.log" 2>&1; then
    echo "$label:1: [pgschema/apply] pgschema cannot apply this schema to an empty database. Fix: make the schema valid for PostgreSQL; pgschema said:"
    sed 's/^/    /' "$tmp/apply.log"
    return 1
  fi
}

plan() {
  local list base short context schema plan status=0
  list=$(selected)
  [ -n "$list" ] || return 0
  base=$(tools/scripts/merge-base.sh)
  if [ -z "$base" ]; then
    echo "no base commit (CHECK_BASE=${CHECK_BASE:-origin/main}): nothing to compare"
    return 0
  fi
  short=$(git rev-parse --short "$base")
  create_scratch
  mkdir -p .cache/sql-plan
  for context in $list; do
    schema="$(dir_of "$context")/schema.sql"
    plan=".cache/sql-plan/$context.sql"
    rm -f "$plan"
    if git cat-file -e "$base:$schema" 2> /dev/null; then
      git show "$base:$schema" > "$tmp/base.sql"
      apply_schema "$context" "$tmp/base.sql" "$schema (at $short)" || {
        status=1
        continue
      }
    else
      echo "$schema: new since $short, so the plan creates every object"
    fi
    if ! pgschema_in plan "$context" "$schema" --output-sql "$plan" > "$tmp/plan.log" 2>&1; then
      echo "$schema:1: [pgschema/plan] pgschema cannot plan this schema. Fix: make the schema valid for PostgreSQL; pgschema said:"
      sed 's/^/    /' "$tmp/plan.log"
      status=1
      continue
    fi
    if [ ! -s "$plan" ]; then
      echo "$schema: no change since $short"
      continue
    fi
    if squawk --config=.squawk.plan.toml --reporter=json "$plan" > "$tmp/squawk.json" 2> "$tmp/squawk.err"; then
      echo "$schema: the plan passes; see $plan"
      continue
    fi
    status=1
    if jq -e 'type == "array" and length > 0' "$tmp/squawk.json" > /dev/null 2>&1; then
      jq -r --arg schema "$schema" --rawfile sql "$plan" -f tools/squawk/plan-findings.jq "$tmp/squawk.json"
    else
      cat "$tmp/squawk.json" "$tmp/squawk.err"
    fi
  done
  return "$status"
}

case "$kind" in
  plan) plan ;;
  *)
    echo "usage: db-check.sh plan" >&2
    exit 2
    ;;
esac
