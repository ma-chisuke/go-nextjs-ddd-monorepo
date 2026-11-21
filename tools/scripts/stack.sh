#!/usr/bin/env bash
# The whole local system in Docker Compose (TPL-0017): PostgreSQL with the
# schema of every bounded context, AWS Emulator with the event bus and the
# queues, Mailpit, the services of every context, built from
# backend/Dockerfile, and the frontend, built from frontend/Dockerfile.
#
# Usage: stack.sh up       start PostgreSQL and apply every schema (db.sh up),
#                          start AWS Emulator and create the bus and the
#                          queues in it (aws.sh apply), then build and start
#                          every service of compose.yaml and wait until each
#                          one is healthy
#        stack.sh down     stop and remove the containers; the data of
#                          PostgreSQL stays
#        stack.sh build    build the images of the product: one of every
#                          context, and the frontend
#        stack.sh images   print "<service> <image>" for each of those images
#        stack.sh smoke    with PostgreSQL and AWS Emulator running (db.sh
#                          server, aws.sh server), apply every schema, create
#                          the bus and the queues, start Mailpit and the
#                          services of the product, run the smoke test
#                          against them, and stop the services that were not
#                          running before
#        stack.sh e2e      the same, with the end-to-end test in a browser
#                          (Playwright, frontend/e2e) instead of the smoke test
#
# Each service publishes its port on the loopback address; compose.yaml holds
# the defaults (POSTGRES_PORT, AWS_PORT, MAILPIT_PORT, FACILITY_PORT,
# RESERVATION_PORT, WEB_PORT).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# The services of the product, which compose.yaml builds from this
# repository, with their images: "<service> <image>" per line. They are the
# services of the bounded contexts (backend/Dockerfile) and the frontend
# (frontend/Dockerfile). AWS Emulator is built too, from infra/local/, but it
# is a tool of the local environment, not a part of the product:
# tools/scripts/aws.sh starts it, and nothing ships it.
product_services() {
  docker compose config --format json \
    | jq -r --arg local "$PWD/infra/local/" \
      '.services | to_entries[]
       | select(.value.build != null and (.value.build.context | startswith($local) | not))
       | "\(.key) \(.value.image)"'
}

# The images of the product, once each: the API and the worker of a context
# are two services that run one image (TPL-0014).
images() {
  product_services | sort -u -k 2,2
}

built_services() {
  product_services | cut -d ' ' -f 1
}

# Build and start services (all of them without arguments), and wait until
# each one is healthy.
start() {
  if ! docker compose up --build --detach --wait --wait-timeout 120 --quiet-pull "$@"; then
    echo "stack: a service did not become healthy; its last log lines:" >&2
    docker compose ps --all >&2
    built_services | xargs docker compose logs --no-color --tail 30 >&2
    exit 1
  fi
}

# The relay and the worker need the bus and the queue when they start, so the
# emulator comes up first, and Terraform fills it before the services start.
#
# A service that calls AWS reads the CA of the emulator once, when it starts,
# and the emulator makes a new CA at every start. Compose leaves a running
# service alone when its own settings did not change, so after the emulator
# started anew, the services that mount its CA are restarted.
up() {
  local before after listed client
  local clients=()
  tools/scripts/db.sh up
  before=$(emulator_start)
  start aws
  after=$(emulator_start)
  tools/scripts/aws.sh apply
  start
  if [ "$before" != "$after" ]; then
    listed=$(emulator_clients)
    while IFS= read -r client; do
      [ -z "$client" ] || clients+=("$client")
    done <<< "$listed"
    if [ "${#clients[@]}" -gt 0 ]; then
      docker compose restart "${clients[@]}" > /dev/null
      start --no-deps "${clients[@]}"
    fi
  fi
}

# Which container of the emulator runs, and since when; empty without one.
emulator_start() {
  local container
  container=$(docker compose ps --quiet aws)
  if [ -n "$container" ]; then
    docker inspect --format '{{.Id}} {{.State.StartedAt}}' "$container"
  fi
}

# The services that mount the volume with the CA of the emulator.
emulator_clients() {
  docker compose config --format json \
    | jq -r '.services | to_entries[]
       | select(.key != "aws" and any(.value.volumes[]?; .source == "aws-emulator"))
       | .key'
}

# The base URL of a service on this host, from the port that Compose published
# for the port of its container (8080 unless given).
url_of() {
  local address
  address=$(docker compose port "$1" "${2:-8080}")
  echo "http://${address/0.0.0.0/127.0.0.1}"
}

# Run a test against the local system: with_system <function>. make has
# started PostgreSQL and AWS Emulator already (db-server, aws-server), and
# other checks use them at the same time, so this brings the schemas, the
# bus, and the queues up to date, and starts the other services without
# touching those two containers (--no-deps): Compose would recreate them when
# another checkout started them.
#
# The smoke test and the end-to-end test both do this, and in `make check-all`
# they start at the same time. Each one stops the services that it started, so
# they take turns: the second one waits for the lock of the first.
with_system() {
  local running listed service
  local services=()
  listed=$(built_services)
  while IFS= read -r service; do
    [ -z "$service" ] || services+=("$service")
  done <<< "$listed"
  services+=(mailpit)
  lock
  running=$(docker compose ps --status running --services)
  started=()
  for service in "${services[@]}"; do
    if ! grep -qxF "$service" <<< "$running"; then
      started+=("$service")
    fi
  done
  trap finish EXIT
  tools/scripts/db.sh init > /dev/null
  tools/scripts/aws.sh apply
  start --no-deps "${services[@]}"
  "$@"
}

# A lock for the tests that start and stop services, in a directory that
# every checkout on this machine shares, because they share the Compose
# project too. A lock whose process is gone is taken over.
lock() {
  lock_dir="${TMPDIR:-/tmp}"
  lock_dir="${lock_dir%/}/stack.sh.lock"
  local tries=0 owner
  until mkdir "$lock_dir" 2> /dev/null; do
    owner=$(cat "$lock_dir/pid" 2> /dev/null || true)
    if [ -n "$owner" ] && ! kill -0 "$owner" 2> /dev/null; then
      rm -rf "$lock_dir"
      continue
    fi
    tries=$((tries + 1))
    if [ "$tries" -gt 4500 ]; then
      echo "stack: another stack.sh has held $lock_dir for 15 minutes; remove the directory if none runs" >&2
      exit 1
    fi
    sleep 0.2
  done
  echo "$$" > "$lock_dir/pid"
}

# Stop the services that the test started, and give the lock back;
# PostgreSQL and AWS Emulator keep running.
finish() {
  if [ "${#started[@]}" -gt 0 ]; then
    docker compose rm --stop --force "${started[@]}" > /dev/null 2>&1 || true
  fi
  rm -rf "$lock_dir"
}

# One scenario across the APIs over HTTP, and the mail that follows from it
# (backend/test/smoke).
smoke() {
  (
    cd backend
    FACILITY_URL=$(url_of facility) RESERVATION_URL=$(url_of reservation) MAILPIT_URL=$(url_of mailpit 8025) \
      go test -tags smoke -count=1 -v ./test/smoke/
  )
}

# The scenarios of a person in a browser, with Playwright (frontend/e2e). The
# browser is Chromium alone, as Playwright pins it, kept in .cache/ and
# downloaded when it is missing.
e2e() {
  export PLAYWRIGHT_BROWSERS_PATH="$PWD/.cache/ms-playwright"
  local web mailpit
  web=$(url_of web 3000)
  mailpit=$(url_of mailpit 8025)
  (
    cd frontend
    # The whole browser, without the headless shell (playwright.config.ts).
    node_modules/.bin/playwright install --no-shell chromium > /dev/null
    WEB_URL=$web MAILPIT_URL=$mailpit node_modules/.bin/playwright test
  )
}

case "${1:-}" in
  up)
    up
    ;;
  down)
    docker compose down --remove-orphans
    ;;
  build)
    built_services | xargs docker compose build --quiet
    ;;
  images)
    images
    ;;
  smoke)
    with_system smoke
    ;;
  e2e)
    with_system e2e
    ;;
  *)
    echo "usage: stack.sh up|down|build|images|smoke|e2e" >&2
    exit 2
    ;;
esac
