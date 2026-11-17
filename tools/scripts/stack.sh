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
#        stack.sh build    build the image of every service that compose.yaml
#                          builds from this repository
#        stack.sh images   print "<service> <image>" for each of those services
#
# Each service publishes its port on the loopback address; compose.yaml holds
# the defaults (POSTGRES_PORT, FACILITY_PORT, RESERVATION_PORT).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# The services that compose.yaml builds from this repository, with their
# images: "<service> <image>" per line.
images() {
  docker compose config --format json \
    | jq -r '.services | to_entries[] | select(.value.build != null) | "\(.key) \(.value.image)"'
}

built_services() {
  images | cut -d ' ' -f 1
}

up() {
  tools/scripts/db.sh up
  if ! docker compose up --build --detach --wait --wait-timeout 120 --quiet-pull; then
    echo "stack: a service did not become healthy; its last log lines:" >&2
    docker compose ps --all >&2
    # shellcheck disable=SC2046 # one service per word
    docker compose logs --no-color --tail 30 $(built_services) >&2
    exit 1
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
    # shellcheck disable=SC2046 # one service per word
    docker compose build --quiet $(built_services)
    ;;
  images)
    images
    ;;
  *)
    echo "usage: stack.sh up|down|build|images" >&2
    exit 2
    ;;
esac
