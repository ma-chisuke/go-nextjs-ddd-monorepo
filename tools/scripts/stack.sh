#!/usr/bin/env bash
# The whole local system in Docker Compose (TPL-0017): PostgreSQL with the
# schema of every bounded context, and the service of every context, built
# from backend/Dockerfile.
#
# Usage: stack.sh up       start PostgreSQL and apply every schema (db.sh up),
#                          then build and start every service of compose.yaml
#                          and wait until each one is healthy
#        stack.sh down     stop and remove the containers; the data of
#                          PostgreSQL stays
#        stack.sh build    build the image of the service of every context
#                          (the services that compose.yaml builds from backend/)
#        stack.sh images   print "<service> <image>" for each of those services
#        stack.sh smoke    with PostgreSQL running (db.sh server), apply every
#                          schema, start the services of the contexts, run the
#                          smoke test against them, and stop the services that
#                          were not running before
#
# Each service publishes its port on the loopback address; compose.yaml holds
# the defaults (POSTGRES_PORT, FACILITY_PORT, RESERVATION_PORT).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# The services of the bounded contexts, which compose.yaml builds from
# backend/Dockerfile, with their images: "<service> <image>" per line. AWS
# Emulator is built too, but it is a tool of the local environment, not a
# part of the product: tools/scripts/aws.sh starts it, and nothing ships it.
images() {
  docker compose config --format json \
    | jq -r --arg backend "$PWD/backend" \
      '.services | to_entries[] | select(.value.build.context == $backend) | "\(.key) \(.value.image)"'
}

built_services() {
  images | cut -d ' ' -f 1
}

# Build and start services (all of them without arguments), and wait until
# each one is healthy.
start() {
  if ! docker compose up --build --detach --wait --wait-timeout 120 --quiet-pull "$@"; then
    echo "stack: a service did not become healthy; its last log lines:" >&2
    docker compose ps --all >&2
    # shellcheck disable=SC2046 # one service per word
    docker compose logs --no-color --tail 30 $(built_services) >&2
    exit 1
  fi
}

up() {
  tools/scripts/db.sh up
  start
}

# The base URL of a service on this host, from the port that Compose published.
url_of() {
  local address
  address=$(docker compose port "$1" 8080)
  echo "http://${address/0.0.0.0/127.0.0.1}"
}

# `make smoke` has started PostgreSQL already (db-server), and other checks
# use it at the same time, so the smoke test brings the schemas up to date
# and starts the services without touching the container of PostgreSQL
# (--no-deps): Compose would recreate it when another checkout started it.
smoke() {
  local running service services
  services=$(built_services)
  running=$(docker compose ps --status running --services)
  started=()
  for service in $services; do
    if ! grep -qxF "$service" <<< "$running"; then
      started+=("$service")
    fi
  done
  trap stop_started EXIT
  tools/scripts/db.sh init > /dev/null
  # shellcheck disable=SC2086 # one service per word
  start --no-deps $services
  (
    cd backend
    FACILITY_URL=$(url_of facility) RESERVATION_URL=$(url_of reservation) \
      go test -tags smoke -count=1 -v ./test/smoke/
  )
}

# Stop the services that the smoke test started; PostgreSQL keeps running.
stop_started() {
  if [ "${#started[@]}" -gt 0 ]; then
    docker compose rm --stop --force "${started[@]}" > /dev/null 2>&1 || true
  fi
}

case "${1:-}" in
  up)
    up
    ;;
  down)
    docker compose down --remove-orphans
    ;;
  build)
    services=$(built_services)
    # shellcheck disable=SC2086 # one service per word
    docker compose build --quiet $services
    ;;
  images)
    images
    ;;
  smoke)
    smoke
    ;;
  *)
    echo "usage: stack.sh up|down|build|images|smoke" >&2
    exit 2
    ;;
esac
