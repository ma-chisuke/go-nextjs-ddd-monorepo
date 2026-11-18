#!/usr/bin/env bash
# The whole local system in Docker Compose (TPL-0017): PostgreSQL with the
# schema of every bounded context, AWS Emulator with the event bus and the
# queues, Mailpit, and the services of every context, built from
# backend/Dockerfile.
#
# Usage: stack.sh up       start PostgreSQL and apply every schema (db.sh up),
#                          start AWS Emulator and create the bus and the
#                          queues in it (aws.sh apply), then build and start
#                          every service of compose.yaml and wait until each
#                          one is healthy
#        stack.sh down     stop and remove the containers; the data of
#                          PostgreSQL stays
#        stack.sh build    build the image of every context (the images that
#                          compose.yaml builds from backend/)
#        stack.sh images   print "<service> <image>" for each of those images
#        stack.sh smoke    with PostgreSQL and AWS Emulator running (db.sh
#                          server, aws.sh server), apply every schema, create
#                          the bus and the queues, start Mailpit and the
#                          services of the contexts, run the smoke test
#                          against them, and stop the services that were not
#                          running before
#
# Each service publishes its port on the loopback address; compose.yaml holds
# the defaults (POSTGRES_PORT, AWS_PORT, MAILPIT_PORT, FACILITY_PORT,
# RESERVATION_PORT).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# The services of the bounded contexts, which compose.yaml builds from
# backend/Dockerfile, with their images: "<service> <image>" per line. AWS
# Emulator is built too, but it is a tool of the local environment, not a
# part of the product: tools/scripts/aws.sh starts it, and nothing ships it.
context_services() {
  docker compose config --format json \
    | jq -r --arg backend "$PWD/backend" \
      '.services | to_entries[] | select(.value.build.context == $backend) | "\(.key) \(.value.image)"'
}

# The images of the contexts, once each: the API and the worker of a context
# are two services that run one image (TPL-0014).
images() {
  context_services | sort -u -k 2,2
}

built_services() {
  context_services | cut -d ' ' -f 1
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

# The relay and the worker need the bus and the queue when they start, so the
# emulator comes up first, and Terraform fills it before the services start.
up() {
  tools/scripts/db.sh up
  start aws
  tools/scripts/aws.sh apply
  start
}

# The base URL of a service on this host, from the port that Compose published
# for the port of its container (8080 unless given).
url_of() {
  local address
  address=$(docker compose port "$1" "${2:-8080}")
  echo "http://${address/0.0.0.0/127.0.0.1}"
}

# `make smoke` has started PostgreSQL and AWS Emulator already (db-server,
# aws-server), and other checks use them at the same time, so the smoke test
# brings the schemas, the bus, and the queues up to date, and starts the
# other services without touching those two containers (--no-deps): Compose
# would recreate them when another checkout started them.
smoke() {
  local running service services
  services="$(built_services) mailpit"
  running=$(docker compose ps --status running --services)
  started=()
  for service in $services; do
    if ! grep -qxF "$service" <<< "$running"; then
      started+=("$service")
    fi
  done
  trap stop_started EXIT
  tools/scripts/db.sh init > /dev/null
  tools/scripts/aws.sh apply
  # shellcheck disable=SC2086 # one service per word
  start --no-deps $services
  (
    cd backend
    FACILITY_URL=$(url_of facility) RESERVATION_URL=$(url_of reservation) MAILPIT_URL=$(url_of mailpit 8025) \
      go test -tags smoke -count=1 -v ./test/smoke/
  )
}

# Stop the services that the smoke test started; PostgreSQL and AWS Emulator
# keep running.
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
