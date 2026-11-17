#!/usr/bin/env bash
# The local database: PostgreSQL in Docker Compose, with the schema of every
# bounded context applied by pgschema (TPL-0010, TPL-0017).
#
# Usage: db.sh up       start PostgreSQL, then do what `init` does
#        db.sh server   start PostgreSQL unless it already accepts connections
#        db.sh init     run db/bootstrap.sql and db/local.sql, apply every schema
#        db.sh apply    apply the schema.sql of every context
#        db.sh plan     print the DDL that `apply` would run, per context
#        db.sh down     stop PostgreSQL and delete its data
#
# `up` lets Compose recreate the container when compose.yaml changed, and
# Compose also recreates it when another checkout (a Git worktree) started
# it, which cuts every open connection. `server` leaves a running server
# alone and changes no database, so the targets of `make check-all` and the
# copies of the self-test use it; `init` then needs no Compose but `exec`.
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
#
# `up`, `init`, and `apply` take a lock on the database, so that two runs at
# once, such as `make test-integration` and `make smoke` in `make check-all`,
# do not apply a schema twice at the same time; the second one waits.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

host="${PGHOST:-127.0.0.1}"
port="${POSTGRES_PORT:-55432}"
database="${PGDATABASE:-app}"

# A lock per database, in a directory that every checkout on this machine
# shares, because they share the database too. A lock whose process is gone
# is taken over.
lock() {
  local dir="${TMPDIR:-/tmp}"
  dir="${dir%/}/db.sh-$host-$port-$database.lock"
  local tries=0 owner
  until mkdir "$dir" 2> /dev/null; do
    owner=$(cat "$dir/pid" 2> /dev/null || true)
    if [ -n "$owner" ] && ! kill -0 "$owner" 2> /dev/null; then
      rm -rf "$dir"
      continue
    fi
    tries=$((tries + 1))
    if [ "$tries" -gt 1500 ]; then
      echo "db: another db.sh has held $dir for 5 minutes; remove the directory if none runs" >&2
      exit 1
    fi
    sleep 0.2
  done
  echo "$$" > "$dir/pid"
  # shellcheck disable=SC2064 # the path is fixed now
  trap "rm -rf '$dir'" EXIT
}

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

# The image runs db/bootstrap.sql on a server that listens on a socket only,
# and restarts it afterwards, so the healthcheck of compose.yaml can pass
# before that restart. Only the server that stays listens on TCP.
ready() {
  docker compose exec -T postgres pg_isready --quiet --host 127.0.0.1 --username postgres --dbname "$database" > /dev/null 2>&1
}

start() {
  docker compose up --detach --wait postgres > /dev/null 2>&1 || {
    docker compose up --detach --wait postgres
    exit 1
  }
  local tries=0
  until ready; do
    tries=$((tries + 1))
    if [ "$tries" -ge 60 ]; then
      echo "db: PostgreSQL did not accept connections within 60 seconds" >&2
      docker compose logs --tail 20 postgres >&2
      exit 1
    fi
    sleep 1
  done
}

# Roles belong to the whole server, so two runs of db/bootstrap.sql, even in
# two databases, can change the same role at once, and PostgreSQL fails one
# of them ("tuple concurrently updated"). The files can run again, so a
# failed run is tried twice more.
psql_file() {
  local tries=0 out
  until out=$(docker compose exec -T postgres psql --username postgres --dbname "$database" \
    --set ON_ERROR_STOP=1 --quiet --file - < "$1" 2>&1); do
    tries=$((tries + 1))
    if [ "$tries" -ge 3 ]; then
      printf '%s\n' "$out" >&2
      return 1
    fi
    sleep 1
  done
  printf '%s' "$out" | { grep -v 'NOTICE:' || true; }
}

apply() {
  local context
  for context in $(contexts); do
    pgschema_for apply "$context" --auto-approve > /dev/null
    echo "db: the schema $context is up to date"
  done
}

init() {
  # The image runs these files only when it creates the data directory;
  # running them again brings an older local database up to date.
  psql_file db/bootstrap.sql
  psql_file db/local.sql
  apply
}

case "${1:-}" in
  up)
    lock
    start
    init
    ;;
  server)
    ready || start
    ;;
  init)
    lock
    init
    ;;
  apply)
    lock
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
    echo "usage: db.sh up|server|init|apply|plan|down" >&2
    exit 2
    ;;
esac
