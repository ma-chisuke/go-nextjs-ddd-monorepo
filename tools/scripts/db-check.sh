#!/usr/bin/env bash
# The checks and the documents that need PostgreSQL. Each run works in a
# scratch database of the local server, which it creates and drops, so it
# never changes the schemas in the database app (TPL-0009).
#
# Usage: db-check.sh plan        lint the DDL of each schema change (TPL-0010)
#        db-check.sh vet         sqlc vet on the queries of each context (TPL-0011)
#        db-check.sh docs-lint   tbls lint, and fail on out-of-date documents (TPL-0023)
#        db-check.sh docs        write docs/generated/db/<context>/ for every context
#
# plan, vet, and docs-lint read the paths in scope on stdin, one per line
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
# (127.0.0.1), POSTGRES_PORT (15432), DB_OWNER_PASSWORD_<CONTEXT>, and
# PGSCHEMA_PLAN_USER and PGSCHEMA_PLAN_PASSWORD.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

host="${PGHOST:-127.0.0.1}"
port="${POSTGRES_PORT:-15432}"
sslmode="${PGSSLMODE:-disable}"
docs_root=docs/generated/db
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

# The address of the scratch database for the owner role of a context.
owner_dsn() {
  printf 'postgresql://%s_owner:%s@%s:%s/%s?sslmode=%s' "$1" "$(owner_password "$1")" "$host" "$port" "$scratch" "$sslmode"
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

# The line of a table, or of a column in it, in a schema.sql; 1 when not found.
line_of() {
  awk -v table="$2" -v column="${3:-}" '
    BEGIN { pattern = "^create table (if not exists )?([a-z0-9_\"]+\\.)?\"?" table "\"?[ (]" }
    !inside && tolower($0) ~ pattern {
      if (column == "") { print NR; found = 1; exit }
      inside = 1; start = NR; next
    }
    inside && /^\)/ { print start; found = 1; exit }
    inside && ($1 == column || $1 == "\"" column "\"") { print NR; found = 1; exit }
    END { if (!found) print 1 }
  ' "$1"
}

plan() {
  local list base short context schema plan allow allow_file findings status=0
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
    squawk --config=.squawk.plan.toml --reporter=json "$plan" > "$tmp/squawk.json" 2> "$tmp/squawk.err" || true
    if ! jq -e 'type == "array"' "$tmp/squawk.json" > /dev/null 2>&1; then
      cat "$tmp/squawk.json" "$tmp/squawk.err"
      status=1
      continue
    fi
    # The dangerous statements that a person reviewed for this change.
    allow_file="$(dir_of "$context")/schema-changes.allow"
    allow=""
    if [ -f "$allow_file" ]; then
      allow=$(cat "$allow_file")
    fi
    findings=$(jq -r --arg schema "$schema" --rawfile sql "$plan" --arg allow "$allow" --arg allowfile "$allow_file" \
      -f tools/squawk/plan-findings.jq "$tmp/squawk.json")
    if [ -z "$findings" ]; then
      echo "$schema: the plan passes; see $plan"
    else
      printf '%s\n' "$findings"
      status=1
    fi
  done
  return "$status"
}

# The fix that each rule of sqlc vet needs.
vet_fix() {
  case "$1" in
    sqlc/db-prepare) echo "change the query until PostgreSQL prepares it against schema.sql" ;;
    *) echo "change the query so that it follows the rule $1 in sqlc.yaml" ;;
  esac
}

vet() {
  local list context dir config line where status=0
  local finding='^([^:]+): ([A-Za-z0-9_]+): ([^:]+): (.*)$'
  list=$(selected)
  [ -n "$list" ] || return 0
  create_scratch
  for context in $list; do
    dir=$(dir_of "$context")
    config="$dir/sqlc.yaml"
    [ -f "$config" ] || continue
    if ! grep -Eq '^[[:space:]]*-[[:space:]]*sqlc/db-prepare[[:space:]]*$' "$config"; then
      echo "$config:1: [sqlc/no-db-prepare] sqlc vet does not prepare the queries of $context in a database. Fix: copy the settings database, analyzer, and rules from the sqlc.yaml of another context. See TPL-0011."
      status=1
      continue
    fi
    apply_schema "$context" "$dir/schema.sql" || {
      status=1
      continue
    }
    if (cd "$dir" && SQLC_VET_DATABASE_URI="$(owner_dsn "$context")" sqlc vet) > "$tmp/vet.log" 2>&1; then
      continue
    fi
    status=1
    # sqlc vet prints "<file>: <query>: <rule>: <message>"; the line is that
    # of the name of the query.
    while IFS= read -r line; do
      if [[ $line =~ $finding ]]; then
        where=$(grep -rnE --include='*.sql' -- "^-- name: ${BASH_REMATCH[2]}( |$)" "$dir" | head -n 1 | cut -d: -f1,2)
        echo "${where:-$config:1}: [${BASH_REMATCH[3]}] ${BASH_REMATCH[2]}: ${BASH_REMATCH[4]}. Fix: $(vet_fix "${BASH_REMATCH[3]}"). See TPL-0011."
      else
        echo "$line"
      fi
    done < "$tmp/vet.log"
  done
  return "$status"
}

# The schema documents of each context in the list $2, written under $1.
write_docs() {
  local out="$1" list="$2" context
  for context in $list; do
    apply_schema "$context" "$(dir_of "$context")/schema.sql" || return 1
    TBLS_DSN="$(owner_dsn "$context")" TBLS_NAME="$context" TBLS_DOC_PATH="$out/$context" \
      tbls doc --config .tbls.yml > "$tmp/tbls.log" 2>&1 || {
      cat "$tmp/tbls.log"
      return 1
    }
  done
}

docs() {
  local context list
  list=$(contexts)
  create_scratch
  write_docs "$tmp/docs" "$list"
  rm -rf "$docs_root"
  mkdir -p "$docs_root"
  for context in $list; do
    cp -R "$tmp/docs/$context" "$docs_root/$context"
    echo "db: wrote $docs_root/$context"
  done
}

# tbls lint prints "<schema>.<table>[.<column>]: <message>".
lint_finding() {
  local context="$1" line="$2" schema subject table column rule fix
  schema="$(dir_of "$context")/schema.sql"
  subject=${line%%: *}
  subject=${subject#"$context".}
  table=${subject%%.*}
  column=''
  if [ "$subject" != "$table" ]; then
    column=${subject#*.}
  fi
  case "$line" in
    *': table comment required.'*)
      rule=requireTableComment
      fix="add COMMENT ON TABLE $table IS '...' to $schema"
      ;;
    *': column comment required.'*)
      rule=requireColumnComment
      fix="add COMMENT ON COLUMN $table.$column IS '...' to $schema"
      ;;
    *'foreign key columns do not have an index'*)
      rule=requireForeignKeyIndex
      fix="add an index whose first columns are those of the foreign key to $schema"
      ;;
    *)
      rule=lint
      fix="change $schema so that tbls lint passes (.tbls.yml)"
      ;;
  esac
  echo "$schema:$(line_of "$schema" "$table" "$column"): [tbls/$rule] ${line%.}. Fix: $fix. See TPL-0023."
}

docs_lint() {
  local list context line file dir name status=0
  list=$(selected)
  [ -n "$list" ] || return 0
  create_scratch
  for context in $list; do
    apply_schema "$context" "$(dir_of "$context")/schema.sql" || {
      status=1
      continue
    }
    if ! TBLS_DSN="$(owner_dsn "$context")" TBLS_NAME="$context" TBLS_DOC_PATH="$tmp/docs/$context" \
      tbls lint --config .tbls.yml > "$tmp/lint.log" 2>&1; then
      status=1
      while IFS= read -r line; do
        case "$line" in
          '' | *' detected') ;;
          "$context".*': '*) lint_finding "$context" "$line" ;;
          *) echo "$line" ;;
        esac
      done < "$tmp/lint.log"
    fi
    TBLS_DSN="$(owner_dsn "$context")" TBLS_NAME="$context" TBLS_DOC_PATH="$tmp/docs/$context" \
      tbls doc --config .tbls.yml > "$tmp/tbls.log" 2>&1 || {
      cat "$tmp/tbls.log"
      status=1
      continue
    }
    # Every file that tbls writes now, and every file in the repository.
    for file in $({ (cd "$tmp/docs/$context" && find . -type f) && { [ ! -d "$docs_root/$context" ] || (cd "$docs_root/$context" && find . -type f); }; } | sort -u); do
      file=${file#./}
      if ! cmp -s "$tmp/docs/$context/$file" "$docs_root/$context/$file"; then
        echo "$docs_root/$context/$file:1: [gen/stale] the schema document is not what tbls writes from $(dir_of "$context")/schema.sql. Fix: run make gen-db-docs. See TPL-0023."
        status=1
      fi
    done
  done
  # Documents of a context that no longer exists.
  for dir in "$docs_root"/*/; do
    [ -d "$dir" ] || continue
    name=$(basename "$dir")
    if ! contexts | grep -Fxq "$name"; then
      echo "$docs_root/$name:1: [gen/stale] there is no context $name. Fix: run make gen-db-docs. See TPL-0023."
      status=1
    fi
  done
  return "$status"
}

case "$kind" in
  plan) plan ;;
  vet) vet ;;
  docs-lint) docs_lint ;;
  docs) docs ;;
  *)
    echo "usage: db-check.sh plan|vet|docs-lint|docs" >&2
    exit 2
    ;;
esac
