#!/usr/bin/env bash
# The image that prepares the database of an environment at deploy time
# (db/Dockerfile, db/prepare.sh), built and run against the local PostgreSQL
# of Docker Compose the way an ECS task runs it against Aurora: as a master
# user that owns the database and may create roles, but is not a superuser
# (TPL-0009, TPL-0016).
#
# Usage: db-image.sh build   build the image go-nextjs-ddd-monorepo/db:local
#        db-image.sh test    build it, run it twice on a database of its own,
#                            and check what it prepared
#
# `test` needs the running PostgreSQL of compose.yaml (`db.sh server`), and
# changes nothing that the local system relies on:
# - It creates a database and a master user of its own, named after its
#   process, and drops both at the end.
# - Roles belong to the whole server, so the roles of the contexts exist
#   already: db/bootstrap.sql created them as the superuser. The test grants
#   its master user the ADMIN OPTION on them, which the master user of Aurora
#   holds because it created them.
# - It passes the passwords that db/local.sql sets, the name of each role, so
#   no role changes its password; db/local.sql runs once more at the end.
#
# The container reaches PostgreSQL on the network of the Compose project, at
# the name of its service, with a read-only root file system. Then the test
# checks that
# - a second run succeeds and changes nothing: the dump of the schema of the
#   database, and the roles of the contexts with their memberships, are what
#   the first run left,
# - the runtime role of each context signs in with its password, reads every
#   table of its own schema, and cannot read a table of another context,
# - no output of the image holds the password of the master user, and
# - the image holds the pgschema that mise.toml pins.
# One finding per line.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

image=go-nextjs-ddd-monorepo/db:local

build() {
  local log
  log=$(mktemp)
  if ! docker build --quiet --file db/Dockerfile --tag "$image" . > "$log" 2>&1; then
    echo "db/Dockerfile:1: [db-image/build] the image does not build; docker said:"
    sed 's/^/    /' "$log"
    rm -f "$log"
    return 1
  fi
  rm -f "$log"
}

contexts() {
  local file
  for file in backend/internal/*/internal/adapter/postgres/schema.sql; do
    [ -f "$file" ] || continue
    file=${file#backend/internal/}
    printf '%s\n' "${file%%/*}"
  done
}

upper() {
  printf '%s' "$1" | tr '[:lower:]' '[:upper:]'
}

schema_file() {
  printf 'backend/internal/%s/internal/adapter/postgres/schema.sql' "$1"
}

# psql as the superuser, inside the container of PostgreSQL.
psql_su() {
  docker compose exec -T postgres psql --no-psqlrc --quiet --set ON_ERROR_STOP=1 --username postgres "$@"
}

# psql as a role of a context, with its password: inside the container, the
# name of the service is an address of the network of Compose, where
# PostgreSQL asks for the password (only the loopback address is trusted).
psql_as() {
  local role="$1"
  shift
  docker compose exec -T -e PGPASSWORD="$role" postgres psql --no-psqlrc --quiet \
    --host postgres --username "$role" --dbname "$name" "$@"
}

name=""
tmp=""
cleanup() {
  if [ -n "$name" ]; then
    docker rm --force "$name" > /dev/null 2>&1 || true
    psql_su --dbname postgres --command "DROP DATABASE IF EXISTS $name WITH (FORCE)" > /dev/null 2>&1 || true
    psql_su --dbname postgres --command "DROP ROLE IF EXISTS $name" > /dev/null 2>&1 || true
    psql_su --dbname postgres --file - < db/local.sql > /dev/null 2>&1 || true
  fi
  if [ -n "$tmp" ]; then
    rm -rf "$tmp"
  fi
}

# The database and its master user: a role with CREATEROLE and CREATEDB that
# owns the database, like the master user of Aurora. On the roles of the
# contexts that exist already, it gets what PostgreSQL gives the creator of a
# role: the ADMIN OPTION, without inheriting the privileges of the role and
# without SET ROLE to it.
create_database() {
  local roles='' context
  for context in $(contexts); do
    roles="$roles, '${context}_owner', '${context}_runtime'"
  done
  psql_su --dbname postgres --file - > /dev/null << EOF
CREATE ROLE $name LOGIN CREATEROLE CREATEDB PASSWORD '$master_password';
CREATE DATABASE $name OWNER $name;
SELECT format('GRANT %I TO %I WITH ADMIN TRUE, INHERIT FALSE, SET FALSE', rolname, '$name')
FROM pg_roles WHERE rolname IN (${roles#, }) \gexec
EOF
}

# Run the image once, as the ECS task does; its output goes to the file given.
run_image() {
  local out="$1" context network
  local environment=(-e PGHOST=postgres -e PGPORT=5432 -e "PGDATABASE=$name" -e PGSSLMODE=disable
    -e "PGUSER=$name" -e PGPASSWORD -e PGCONNECT_TIMEOUT=10 -e DB_CONNECT_TRIES=3)
  for context in $(contexts); do
    export "DB_OWNER_PASSWORD_$(upper "$context")=${context}_owner"
    export "DB_RUNTIME_PASSWORD_$(upper "$context")=${context}_runtime"
    environment+=(-e "DB_OWNER_PASSWORD_$(upper "$context")" -e "DB_RUNTIME_PASSWORD_$(upper "$context")")
  done
  network=$(docker inspect --format '{{range $network, $_ := .NetworkSettings.Networks}}{{println $network}}{{end}}' \
    "$(docker compose ps --quiet postgres)" | sed -n 1p)
  PGPASSWORD="$master_password" docker run --rm --name "$name" --network "$network" \
    --read-only --cap-drop ALL --security-opt no-new-privileges \
    "${environment[@]}" "$image" > "$out" 2>&1
}

# What a run may change: the schema of the database with its owners and
# privileges, and the roles of the contexts with their settings and
# memberships. The hash of a password is left out: setting the same password
# again stores it with a new salt.
fingerprint() {
  local roles='' context
  for context in $(contexts); do
    roles="$roles, '${context}_owner', '${context}_runtime'"
  done
  docker compose exec -T postgres pg_dump --username postgres --schema-only --restrict-key=dbimage "$name"
  psql_su --dbname postgres --tuples-only --no-align << EOF
SELECT rolname, rolsuper, rolinherit, rolcreaterole, rolcreatedb, rolcanlogin, rolconfig
FROM pg_roles WHERE rolname IN (${roles#, }) ORDER BY rolname;
SELECT roleid::regrole, member::regrole, grantor::regrole, admin_option, inherit_option, set_option
FROM pg_auth_members
WHERE roleid::regrole::text IN (${roles#, }) OR member::regrole::text IN (${roles#, })
ORDER BY 1, 2, 3;
EOF
}

# The runtime role of a context reads each table of its schema, and no table
# of another context.
check_runtime() {
  local context="$1" other tables count denied status=0
  tables=$(psql_su --dbname "$name" --tuples-only --no-align \
    --command "SELECT count(*) FROM pg_tables WHERE schemaname = '$context'")
  if [ "$tables" -eq 0 ]; then
    echo "$(schema_file "$context"):1: [db-image/tables] the schema $context has no table after the run"
    return 1
  fi
  if ! psql_as "${context}_runtime" --set ON_ERROR_STOP=1 --file - > "$tmp/read.log" 2>&1 << EOF
SELECT format('SELECT 1 FROM %I.%I LIMIT 1', schemaname, tablename)
FROM pg_tables WHERE schemaname = '$context' \gexec
EOF
  then
    echo "$(schema_file "$context"):1: [db-image/runtime-read] ${context}_runtime cannot sign in with its password, or cannot read a table of its schema; psql said:"
    sed 's/^/    /' "$tmp/read.log"
    status=1
  fi
  for other in $(contexts); do
    [ "$other" != "$context" ] || continue
    count=$(psql_su --dbname "$name" --tuples-only --no-align \
      --command "SELECT count(*) FROM pg_tables WHERE schemaname = '$other'")
    psql_as "${context}_runtime" --set ON_ERROR_STOP=0 --tuples-only --file - > "$tmp/other.log" 2>&1 << EOF || true
SELECT format('SELECT 1 FROM %I.%I LIMIT 1', schemaname, tablename)
FROM pg_tables WHERE schemaname = '$other' \gexec
EOF
    denied=$(grep -c 'permission denied' "$tmp/other.log" || true)
    if [ "$denied" -ne "$count" ]; then
      echo "$(schema_file "$other"):1: [db-image/isolation] ${context}_runtime can read $((count - denied)) of the $count tables of the schema $other; no role may read the schema of another context (TPL-0009)"
      { grep -v 'permission denied' "$tmp/other.log" || true; } | sed 's/^/    /'
      status=1
    fi
  done
  return "$status"
}

# The pgschema of the image is the one that mise.toml pins.
check_pgschema() {
  local want have
  want=$(pgschema --version)
  have=$(docker run --rm --entrypoint pgschema "$image" --version)
  if [ "$have" != "$want" ]; then
    echo "db/Dockerfile:$(grep -n -m 1 'pgschema/releases' db/Dockerfile | cut -d: -f1): [db-image/pgschema-version] the image holds pgschema $have, but mise.toml pins $want. Fix: change the version and the checksums of both downloads to those of mise.lock"
    return 1
  fi
}

test_image() {
  local status=0 run context
  if [ -z "$(docker compose ps --quiet postgres 2> /dev/null)" ]; then
    echo "db-image: PostgreSQL does not run; start it with \`make db-server\`" >&2
    return 1
  fi
  tmp=$(mktemp -d)
  name="db_image_test_$$_${RANDOM}"
  master_password=$(od -An -tx1 -N16 /dev/urandom | tr -d ' \n')
  trap cleanup EXIT
  trap 'exit 130' INT TERM

  build
  check_pgschema || status=1
  create_database

  for run in 1 2; do
    if ! run_image "$tmp/run$run.log"; then
      echo "db/prepare.sh:1: [db-image/run] run $run of the image failed; it said:"
      sed 's/^/    /' "$tmp/run$run.log"
      return 1
    fi
    fingerprint > "$tmp/after$run.txt"
    if grep -qF "$master_password" "$tmp/run$run.log"; then
      echo "db/prepare.sh:1: [db-image/secret] run $run of the image printed the password of the master user"
      status=1
    fi
  done
  if ! diff -u --label "after run 1" --label "after run 2" "$tmp/after1.txt" "$tmp/after2.txt" > "$tmp/diff.txt"; then
    echo "db/prepare.sh:1: [db-image/idempotent] the second run of the image changed the database:"
    sed 's/^/    /' "$tmp/diff.txt"
    status=1
  fi

  for context in $(contexts); do
    check_runtime "$context" || status=1
  done
  if [ "$status" -eq 0 ]; then
    echo "db-image: the image prepared $(contexts | wc -l | tr -d ' ') contexts twice, as a master user that is not a superuser"
  fi
  return "$status"
}

case "${1:-}" in
  build)
    build
    ;;
  test)
    test_image
    ;;
  *)
    echo "usage: db-image.sh build|test" >&2
    exit 2
    ;;
esac
