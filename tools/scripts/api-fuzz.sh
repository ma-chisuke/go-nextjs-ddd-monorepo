#!/usr/bin/env bash
# Layer 5 of the API conformance checks (TPL-0007): Schemathesis generates
# requests from each API document in contracts/http/, sends them to the API of
# its context, and fails on each answer that the document does not allow.
#
# Usage: api-fuzz.sh   (make test-api-fuzz, after db-server)
#
# The script
#   1. builds the command of each context that has an API document,
#      backend/cmd/<context>, from the source of this checkout,
#   2. creates a database of its own anew on the local PostgreSQL, with every
#      schema (db.sh init), so that the APIs start empty and never meet the
#      data of another checkout or of the local system,
#   3. starts each API on 127.0.0.1, on a free port between 20000 and 32767:
#      below the range from which the system hands out ports by itself, and
#      above the ports that Docker Compose publishes. Each API gets the URL of
#      every API, such as FACILITY_API_URL, as in Docker Compose. No relay
#      runs: the APIs write their events to the outbox, and nothing reads it,
#   4. runs Schemathesis on each document against its API, with the settings
#      of tools/schemathesis/schemathesis.toml,
#   5. prints one line per finding, `<document>:<line>: [schemathesis/<check>]
#      <message>`, at the line of the operation (tools/schemathesis/findings.jq),
#      and fails when there is one,
#   6. stops the APIs and drops the database, also when it fails or is
#      interrupted.
#
# Two runs of the same code and the same documents send the same requests and
# report the same findings:
#   - generation is deterministic (schemathesis.toml): Hypothesis takes a fixed
#     seed from each test, keeps no database of examples, and the cache of
#     Schemathesis is off. Each run starts in an empty directory, so no file
#     of an earlier run is read;
#   - the database is new, so the APIs answer the first request of every run
#     in the same state;
#   - one worker sends the requests in order, and the documents run one after
#     the other, so the APIs see the requests in the same order.
# What the APIs make themselves still differs between runs: the id of a new
# room or reservation (a UUID version 7) and the time when it was made, which
# later requests of a sequence send back. The clock also decides whether a
# period of the examples, such as 2026-11-20T10:00:00+09:00, is in the past:
# a booking of it answers 422 either way, for a room that does not exist.
#
# Each document leaves its files in .cache/api-fuzz/<context>/:
#   output.txt      what Schemathesis printed, with a curl command for each
#                   failure
#   events.ndjson   every request and answer, which findings.jq reads
#   api.log         the log of the API, with the error behind each 500
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
root=$PWD

cache=.cache/api-fuzz
bin=$cache/bin
host="${PGHOST:-127.0.0.1}"
port="${POSTGRES_PORT:-15432}"
# A database per checkout, so that two checkouts can run the check at once.
database="api_fuzz_$(printf '%s' "$root" | cksum | cut -d ' ' -f 1)"

# The contexts that have an API document and a command.
contexts=()
for doc in contracts/http/*.openapi.yaml; do
  context=$(basename "$doc" .openapi.yaml)
  if [ -d "backend/cmd/$context" ]; then
    contexts+=("$context")
  fi
done
if [ "${#contexts[@]}" -eq 0 ]; then
  echo "api-fuzz: no API document in contracts/http/ has a command in backend/cmd/"
  exit 0
fi

# One run at a time in a checkout: the runs share the database and the files.
# A lock whose process is gone is taken over.
lock_dir=$cache.lock
lock() {
  local owner
  mkdir -p .cache
  while ! mkdir "$lock_dir" 2> /dev/null; do
    owner=$(cat "$lock_dir/pid" 2> /dev/null || true)
    if [ -z "$owner" ] || kill -0 "$owner" 2> /dev/null; then
      echo "api-fuzz: another run holds $lock_dir; remove the directory if none runs" >&2
      return 1
    fi
    rm -rf "$lock_dir"
  done
  echo "$$" > "$lock_dir/pid"
}
lock

drop_database() {
  docker compose exec -T -e PGOPTIONS='-c client_min_messages=warning' postgres \
    psql --username postgres --dbname postgres --quiet --set ON_ERROR_STOP=1 \
    --command "DROP DATABASE IF EXISTS $database WITH (FORCE)" > /dev/null
}

pids=()
stop_apis() {
  if [ "${#pids[@]}" -gt 0 ]; then
    kill "${pids[@]}" 2> /dev/null || true
    wait "${pids[@]}" 2> /dev/null || true
  fi
  pids=()
}

finish() {
  stop_apis
  drop_database || true
  rm -rf "$lock_dir"
}
trap finish EXIT
trap 'exit 130' INT TERM

# 1. The commands, from the source of this checkout.
mkdir -p "$bin"
packages=()
for context in "${contexts[@]}"; do
  packages+=("./cmd/$context")
done
(cd backend && go build -o "$root/$bin/" "${packages[@]}")

# 2. A new database, with every schema.
drop_database
PGDATABASE=$database tools/scripts/db.sh init > /dev/null

# 3. The APIs. A port that is free when it is chosen can be taken before the
# API listens on it; then every API starts again on new ports.
ports=()
url_of() {
  local i
  for i in "${!contexts[@]}"; do
    if [ "${contexts[$i]}" = "$1" ]; then
      echo "http://127.0.0.1:${ports[$i]}"
    fi
  done
}

free_port() {
  local candidate
  while :; do
    candidate=$((20000 + RANDOM % 12768))
    if ! (exec 3<> "/dev/tcp/127.0.0.1/$candidate") 2> /dev/null; then
      echo "$candidate"
      return
    fi
  done
}

# Succeeds when every API answers its health check within 30 seconds; fails
# when one stops.
wait_for_apis() {
  local i tries
  for i in "${!contexts[@]}"; do
    tries=0
    until HTTP_ADDR="127.0.0.1:${ports[$i]}" "$bin/${contexts[$i]}" healthcheck 2> /dev/null; do
      if ! kill -0 "${pids[$i]}" 2> /dev/null; then
        return 1
      fi
      tries=$((tries + 1))
      if [ "$tries" -ge 60 ]; then
        echo "api-fuzz: ${contexts[$i]} did not answer its health check within 30 seconds; $cache/${contexts[$i]}/api.log has its log" >&2
        exit 1
      fi
      sleep 0.5
    done
  done
}

start_apis() {
  local context i urls=()
  ports=()
  for context in "${contexts[@]}"; do
    ports+=("$(free_port)")
  done
  for context in "${contexts[@]}"; do
    urls+=("$(printf '%s' "$context" | tr '[:lower:]' '[:upper:]')_API_URL=$(url_of "$context")")
  done
  for i in "${!contexts[@]}"; do
    context=${contexts[$i]}
    rm -rf "${cache:?}/$context"
    mkdir -p "$cache/$context"
    env "${urls[@]}" \
      DATABASE_URL="postgres://${context}_runtime:${context}_runtime@$host:$port/$database?sslmode=disable" \
      HTTP_ADDR="127.0.0.1:${ports[$i]}" \
      "$bin/$context" > "$cache/$context/api.log" 2>&1 &
    pids+=("$!")
  done
}

for attempt in 1 2 3; do
  start_apis
  if wait_for_apis; then
    break
  fi
  stop_apis
  if [ "$attempt" -eq 3 ]; then
    echo "api-fuzz: an API stopped as it started; its log:" >&2
    tail -n 5 "$cache"/*/api.log >&2
    exit 1
  fi
done

# The line of each path and each operation of a document, by "<path>" and by
# "<METHOD> <path>", as JSON. The documents are formatted by Prettier, so a
# path is indented by two spaces under `paths:`, and a method by four.
operation_lines() {
  awk '
    /^paths:/ { in_paths = 1; next }
    in_paths && /^[^ #]/ { in_paths = 0 }
    in_paths && /^  [^ #][^:]*:/ { path = $1; sub(/:$/, "", path); gsub(/"/, "", path); print path "\t" NR }
    in_paths && /^    (get|put|post|delete|options|head|patch|trace):/ {
      method = $1; sub(/:$/, "", method); print toupper(method) " " path "\t" NR
    }
  ' "$1" | jq -Rn '[inputs | split("\t") | {(.[0]): (.[1] | tonumber)}] | add // {}'
}

# 4 and 5. Schemathesis on each document, one after the other.
status=0
for context in "${contexts[@]}"; do
  doc="contracts/http/$context.openapi.yaml"
  dir="$cache/$context"
  url=$(url_of "$context")
  rm -rf "$dir/run" "$dir/events.ndjson"
  mkdir -p "$dir/run"
  code=0
  (cd "$dir/run" && schemathesis --config-file "$root/tools/schemathesis/schemathesis.toml" --no-color \
    run "$root/$doc" --url "$url" --report ndjson --report-ndjson-path "$root/$dir/events.ndjson") \
    > "$dir/output.txt" 2>&1 || code=$?
  findings=""
  if [ -f "$dir/events.ndjson" ]; then
    findings=$(jq -r -s --arg doc "$doc" --argjson lines "$(operation_lines "$doc")" --arg base "$url" \
      -f tools/schemathesis/findings.jq "$dir/events.ndjson")
  fi
  if [ -n "$findings" ]; then
    printf '%s\n' "$findings"
  fi
  if [ "$code" -ne 0 ]; then
    status=1
    if [ -z "$findings" ]; then
      # Schemathesis did not finish, such as for a document that it cannot
      # read: its own words say why.
      echo "$doc:1: [schemathesis/run] Schemathesis exited with $code; its output follows"
      grep -v '^[[:space:]]*$' "$dir/output.txt" | tail -n 30
    fi
  fi
done
if [ "$status" -ne 0 ]; then
  echo "api-fuzz: the requests and the answers of each finding, with curl commands, are in $cache/<context>/output.txt; the logs of the APIs in $cache/<context>/api.log"
  exit 1
fi
