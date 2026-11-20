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
#   4. registers rooms through the Facility API, and writes their ids and
#      instants of tomorrow and yesterday into dictionaries of Schemathesis,
#      from which generated bookings draw a room and a period (step 4 below
#      says why and what),
#   5. runs Schemathesis on each document against its API, with the settings
#      of tools/schemathesis/schemathesis.toml,
#   6. prints one line per finding, `<document>:<line>: [schemathesis/<check>]
#      <message>`, at the line of the operation (tools/schemathesis/findings.jq),
#      and fails when there is one,
#   7. stops the APIs and drops the database, also when it fails or is
#      interrupted.
#
# Two runs of the same code and the same documents on the same day, in UTC,
# send the same requests and report the same findings:
#   - generation is deterministic (schemathesis.toml): Hypothesis takes a fixed
#     seed from each test, keeps no database of examples, and the cache of
#     Schemathesis is off. Each run starts in an empty directory, so no file
#     of an earlier run is read;
#   - the database is new, and step 4 registers the same rooms in the same
#     order, so the APIs answer the first request of every run in the same
#     state;
#   - one worker sends the requests in order, and the documents run one after
#     the other, so the APIs see the requests in the same order.
# What differs between runs:
#   - the instants of step 4 depend on the date, so a run on another day sends
#     the same periods, moved by whole days;
#   - what the APIs make themselves: the id of a room or a reservation (a UUID
#     version 7), which the dictionaries and later requests of a sequence send
#     back.
#
# Each document leaves its files in .cache/api-fuzz/<context>/:
#   output.txt      what Schemathesis printed, with a curl command for each
#                   failure
#   events.ndjson   every request and answer, which findings.jq reads
#   api.log         the log of the API, with the error behind each 500
# The dictionaries of step 4 are in .cache/api-fuzz/data/.
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

# 4. The data that generated requests draw from. A request that Schemathesis
# makes up cannot know what the APIs hold: a booking names a room that does
# not exist and meets unknown-room before any rule of a reservation sees it,
# and a made-up period almost never lies on the grid of 15 minutes, in the
# future, and inside the opening hours of a room. So the script registers
# rooms through the Facility API, as any client would, and writes their ids
# and some instants into dictionaries of Schemathesis (libFuzzer format, one
# quoted value per line) in $cache/data/. tools/schemathesis/schemathesis.toml
# draws roomId, startsAt, and endsAt of the Reservation API from them, most of
# the time; the rest of the time Schemathesis makes them up.
#
# room-ids.dict, in this order:
#   - six rooms open around the clock in UTC (00:00 to 23:59) that hold 1,000
#     people, in which any period of one day and any attendees fit;
#   - one such room that holds 1 person: more attendees are over-capacity;
#   - one room open from 09:00 to 18:00 in Tokyo, which every period below
#     misses: tomorrow 09:00 in UTC is 18:00 in Tokyo.
# starts.dict and ends.dict, which bookings draw from, hold instants in UTC of
# tomorrow (T) and yesterday (Y):
#   starts  T 09:00, 09:30, 10:00, 10:30, 11:00; T 10:05; Y 10:00
#   ends    T 13:00, 12:30, 12:00, 11:30, 11:15; T 15:15; T 08:00; Y 11:00
# Each of the first five starts with each of the first five ends is a period
# that the rules accept, of 15 minutes to 4 hours. Every other pair breaks a
# rule: T 10:05 is off the grid (period-misaligned), T 15:15 makes the period
# too long, as does Y 10:00 with an end of T (period-too-long), T 08:00 and
# Y 11:00 end before a start of T (period-out-of-order), and Y 10:00 to
# Y 11:00 is in the past (period-in-past). Hypothesis draws the first and the
# last entry of a list more often than the others: the first ones make a
# booking that the rules accept, and the last ones break a rule. All accepted
# periods share the quarter hour from 11:00 to 11:15, so a room holds one
# confirmed reservation at a time, and the next booking of it meets
# room-already-booked until the reservation is cancelled.
#
# accepted-starts.dict and accepted-ends.dict hold the first five of each,
# which reschedulings draw from: a rescheduling changes the start, the end, or
# both, of a reservation whose period the rules accepted, so that it succeeds
# as often as a booking may; the bookings break the rules.
#
# The instants depend on the date alone, never on the time of day: whenever
# the run starts, the earliest start of tomorrow is at least 9 hours away, so
# a reservation can still be cancelled, and yesterday is past.
data=$cache/data
rm -rf "$data"
mkdir -p "$data"

# register_room <JSON of a new room>: registers the room and prints its id.
register_room() {
  local answer
  if ! answer=$(curl --silent --show-error --fail-with-body --max-time 10 \
    --header 'Content-Type: application/json' --data "$1" "$(url_of facility)/rooms" 2>&1); then
    echo "api-fuzz: Facility did not register the room $1: $answer" >&2
    return 1
  fi
  jq -r .id <<< "$answer"
}

# room <name> <capacity> <openFrom> <openUntil> <timeZone>: the JSON of a new
# room.
room() {
  jq -n -c --arg name "api-fuzz: $1" --argjson capacity "$2" --arg from "$3" --arg until "$4" --arg zone "$5" \
    '{name: $name, capacity: $capacity, openFrom: $from, openUntil: $until, timeZone: $zone}'
}

{
  for i in 1 2 3 4 5 6; do
    register_room "$(room "room $i, open around the clock" 1000 00:00 23:59 UTC)"
  done
  register_room "$(room "a room for 1, open around the clock" 1 00:00 23:59 UTC)"
  register_room "$(room "a room open from 9 to 18 in Tokyo" 1000 09:00 18:00 Asia/Tokyo)"
} | jq -R . > "$data/room-ids.dict"

# instants <day>@<HH:MM>...: each instant in UTC, quoted, on its own line. The
# day counts from today in UTC: 1 is tomorrow, -1 yesterday.
instants() {
  jq -n -r '(now | floor | . - . % 86400) as $today
    | $ARGS.positional[] | split("@") | (.[1] | split(":") | map(tonumber)) as $time
    | $today + (.[0] | tonumber) * 86400 + $time[0] * 3600 + $time[1] * 60
    | todate | tojson' --args "$@"
}
instants 1@09:00 1@09:30 1@10:00 1@10:30 1@11:00 > "$data/accepted-starts.dict"
instants 1@13:00 1@12:30 1@12:00 1@11:30 1@11:15 > "$data/accepted-ends.dict"
{
  cat "$data/accepted-starts.dict"
  instants 1@10:05 -1@10:00
} > "$data/starts.dict"
{
  cat "$data/accepted-ends.dict"
  instants 1@15:15 1@08:00 -1@11:00
} > "$data/ends.dict"

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

# 5 and 6. Schemathesis on each document, one after the other.
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
