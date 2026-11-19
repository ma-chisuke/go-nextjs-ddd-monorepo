#!/usr/bin/env bash
# Prepare the database of an environment for the services, at deploy time.
# One ECS task runs this script in the image of db/Dockerfile before the
# services start; infra/modules/database defines that task. Every step is
# safe to run again.
#
#   1. Wait until the database accepts a connection. A paused Aurora
#      Serverless v2 cluster resumes on the first connection, in about 15
#      seconds, or in 30 or more after a long pause (TPL-0016).
#   2. Run db/bootstrap.sql as the master user: the extensions, the schemas,
#      and the roles of every context (TPL-0009).
#   3. Set the password of each <context>_owner and <context>_runtime role.
#   4. Apply the schema.sql of each context with pgschema, as its owner role,
#      as `tools/scripts/db.sh apply` does locally (TPL-0010). pgschema
#      validates the desired state in a scratch schema of the same database,
#      as the master user.
#
# The environment says where the database is and who signs in, as libpq reads
# it:
#   PGHOST, PGPORT (5432), PGDATABASE, PGSSLMODE (prefer)
#   PGUSER, PGPASSWORD                the master user: a role that owns the
#                                     database and may create roles
#   DB_OWNER_PASSWORD_<CONTEXT>       the password of <context>_owner
#   DB_RUNTIME_PASSWORD_<CONTEXT>     the password of <context>_runtime
#   PGCONNECT_TIMEOUT (30), DB_CONNECT_TRIES (5)
#                                     how long one connection waits, and how
#                                     many times the first one is tried
#
# No password is printed or put on a command line, where the list of
# processes would show it: psql reads the passwords from the environment
# (\getenv), and pgschema from PGPASSWORD and PGSCHEMA_PLAN_PASSWORD.
#
# A context is a directory backend/internal/<context>/ with
# internal/adapter/postgres/schema.sql, as in db.sh; the image holds those
# files at the paths of the repository, next to db/.
set -euo pipefail

cd "$(dirname "$0")/.."

export PGCONNECT_TIMEOUT="${PGCONNECT_TIMEOUT:-30}"
port="${PGPORT:-5432}"
sslmode="${PGSSLMODE:-prefer}"
connect_tries="${DB_CONNECT_TRIES:-5}"

fail() {
  echo "prepare: $*" >&2
  exit 1
}

for variable in PGHOST PGDATABASE PGUSER PGPASSWORD; do
  if [ -z "${!variable:-}" ]; then
    fail "set $variable"
  fi
done
# pgschema signs in to the plan database as the master user.
master_password="$PGPASSWORD"

contexts() {
  local file
  for file in backend/internal/*/internal/adapter/postgres/schema.sql; do
    [ -f "$file" ] || continue
    file=${file#backend/internal/}
    printf '%s\n' "${file%%/*}"
  done
}

# The variable that holds the password of a role: password_variable owner facility.
password_variable() {
  printf 'DB_%s_PASSWORD_%s' "$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')" \
    "$(printf '%s' "$2" | tr '[:lower:]' '[:upper:]')"
}

run_psql() {
  psql --no-psqlrc --quiet --set ON_ERROR_STOP=1 "$@"
}

# Roles belong to the whole server, so two runs at once can change the same
# role, and PostgreSQL then fails one of them ("tuple concurrently updated").
# The steps can run again, so a failed one is tried twice more. Its output is
# shown without the notices of what exists already.
retry() {
  local tries=0 out
  until out=$("$@" 2>&1); do
    tries=$((tries + 1))
    if [ "$tries" -ge 3 ]; then
      printf '%s\n' "$out" >&2
      return 1
    fi
    sleep 2
  done
  printf '%s' "$out" | { grep -v 'NOTICE:' || true; }
}

wait_for_database() {
  local tries=0 out
  until out=$(run_psql --tuples-only --command 'SELECT 1' 2>&1); do
    tries=$((tries + 1))
    if [ "$tries" -ge "$connect_tries" ]; then
      printf '%s\n' "$out" >&2
      fail "the database $PGDATABASE at $PGHOST:$port did not accept a connection in $connect_tries tries"
    fi
    echo "prepare: the database does not accept a connection yet, and may be resuming from a pause (try $tries of $connect_tries)"
    sleep 5
  done
  echo "prepare: the database $PGDATABASE at $PGHOST:$port accepts connections"
}

# One psql script for every role: the password comes from the environment
# into a psql variable, and from there into the statement as a literal.
password_script() {
  local context kind variable
  for context in $(contexts); do
    for kind in owner runtime; do
      variable=$(password_variable "$kind" "$context")
      printf '\\getenv password %s\n' "$variable"
      printf "ALTER ROLE %s_%s PASSWORD :'password';\n" "$context" "$kind"
    done
  done
}

set_passwords() {
  password_script | run_psql --file -
}

check_passwords() {
  local context kind variable
  for context in $(contexts); do
    for kind in owner runtime; do
      variable=$(password_variable "$kind" "$context")
      if [ -z "${!variable:-}" ]; then
        fail "set $variable, the password of the role ${context}_${kind}"
      fi
    done
  done
}

apply_schemas() {
  local context variable file
  for context in $(contexts); do
    variable=$(password_variable owner "$context")
    file="backend/internal/$context/internal/adapter/postgres/schema.sql"
    echo "prepare: pgschema applies $file as ${context}_owner"
    PGPASSWORD="${!variable}" PGSCHEMA_PLAN_PASSWORD="$master_password" pgschema apply \
      --host "$PGHOST" --port "$port" --db "$PGDATABASE" --sslmode "$sslmode" \
      --user "${context}_owner" --schema "$context" --file "$file" \
      --plan-host "$PGHOST" --plan-port "$port" --plan-db "$PGDATABASE" --plan-sslmode "$sslmode" \
      --plan-user "$PGUSER" \
      --no-color --auto-approve
    echo "prepare: the schema $context is up to date"
  done
}

if [ -z "$(contexts)" ]; then
  fail "the image holds no backend/internal/<context>/internal/adapter/postgres/schema.sql"
fi
check_passwords
wait_for_database
retry run_psql --file db/bootstrap.sql
echo "prepare: db/bootstrap.sql ran"
retry set_passwords
echo "prepare: the passwords of the roles are set"
apply_schemas
