#!/usr/bin/env bash
# The local database: PostgreSQL in Docker Compose, with the schema of every
# bounded context applied by pgschema (TPL-0010, TPL-0017).
#
# Usage: db.sh up       start PostgreSQL, run db/bootstrap.sql, apply every schema
#        db.sh apply    apply the schema.sql of every context
#        db.sh plan     print the DDL that `apply` would run, per context
#        db.sh down     stop PostgreSQL and delete its data
#
# A context is a directory backend/internal/<context>/ with
# internal/adapter/postgres/schema.sql. Its schema is named <context>, and
# pgschema connects as the role <context>_owner.
#
# The connection is the local one unless the environment says otherwise:
#   PGHOST (127.0.0.1), POSTGRES_PORT (55432), PGDATABASE (app)
#   DB_OWNER_PASSWORD_<CONTEXT>   password of <context>_owner (default: its name)
#   PGSCHEMA_PLAN_USER/PASSWORD   a role that may create schemas in the database
#                                 where pgschema validates the desired state
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

host="${PGHOST:-127.0.0.1}"
port="${POSTGRES_PORT:-55432}"
database="${PGDATABASE:-app}"

contexts() {
  local file
  for file in backend/internal/*/internal/adapter/postgres/schema.sql; do
    [ -f "$file" ] || continue
    file=${file#backend/internal/}
    printf '%s\n' "${file%%/*}"
  done
}

# pgschema validates the desired state in a scratch schema of a real database
# (the plan database), so that it never downloads a PostgreSQL of its own.
pgschema_for() {
  local command="$1" context="$2"
  shift 2
  local owner="${context}_owner"
  local variable
  variable="DB_OWNER_PASSWORD_$(printf '%s' "$context" | tr '[:lower:]' '[:upper:]')"
  PGPASSWORD="${!variable:-$owner}" pgschema "$command" \
    --host "$host" --port "$port" --db "$database" --sslmode "${PGSSLMODE:-disable}" \
    --user "$owner" --schema "$context" \
    --file "backend/internal/$context/internal/adapter/postgres/schema.sql" \
    --plan-host "$host" --plan-port "$port" --plan-db "$database" --plan-sslmode "${PGSSLMODE:-disable}" \
    --plan-user "${PGSCHEMA_PLAN_USER:-postgres}" --plan-password "${PGSCHEMA_PLAN_PASSWORD:-postgres}" \
    --no-color "$@"
}

psql_file() {
  docker compose exec -T postgres psql --username postgres --dbname "$database" \
    --set ON_ERROR_STOP=1 --quiet --file - < "$1" 2>&1 | { grep -v 'NOTICE:' || true; }
}

apply() {
  local context
  for context in $(contexts); do
    pgschema_for apply "$context" --auto-approve > /dev/null
    echo "db: the schema $context is up to date"
  done
}

case "${1:-}" in
  up)
    docker compose up --detach --wait postgres > /dev/null 2>&1 || {
      docker compose up --detach --wait postgres
      exit 1
    }
    # The image runs these files only when it creates the data directory;
    # running them again brings an older local database up to date.
    psql_file db/bootstrap.sql
    psql_file db/local.sql
    apply
    ;;
  apply)
    apply
    ;;
  plan)
    for context in $(contexts); do
      echo "-- $context"
      pgschema_for plan "$context" --output-sql stdout
    done
    ;;
  down)
    docker compose down --volumes
    ;;
  *)
    echo "usage: db.sh up|apply|plan|down" >&2
    exit 2
    ;;
esac
