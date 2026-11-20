#!/usr/bin/env bash
# The criteria of the task reservation-length (evals/run.sh sources this file
# and calls grade_task in the copy of the repository).
#
# The task adds something to a screen. What it measures: where the parts of a
# screen go (the guide "Add a screen"). The logic is pure, in a module with a
# unit test next to it; the component only shows what it returns; and what a
# person sees is a scenario in a browser. `make check` runs the unit tests,
# but neither `next build` nor the scenarios: the grader builds the frontend
# and reads the page, and can only look for a changed scenario, because it
# runs no browser.
#
# The page reads a room from the Facility API and its reservations from the
# Reservation API. fake-apis.mjs answers both with fixed JSON: one room in
# Tokyo, and three reservations on 20 November 2026, of 90, 45, and 120
# minutes.

room=0193a6f0-1a2b-7c3d-8e4f-00000000e001

logs=.cache/eval-reservation-length
eval_servers=()

# The fake APIs and the frontend stop, whatever happened.
stop_servers() {
  if [ "${#eval_servers[@]}" -gt 0 ]; then
    kill "${eval_servers[@]}" 2> /dev/null || true
    wait "${eval_servers[@]}" 2> /dev/null || true
  fi
  eval_servers=()
}

# The frontend of the copy, built as its image builds it (next build,
# standalone) and started against the fake APIs, gives the page of the room
# on 20 November 2026; its text goes to $logs/page.txt. The HTML holds what
# the server streamed; page-text.mjs turns it into the text that a person
# reads.
read_the_page() {
  local api web tries=0
  own_dependencies || return 1
  if ! (cd frontend && node_modules/.bin/next build) > "$logs/build.log" 2>&1; then
    echo "next build failed; the end of its output:"
    tail -n 3 "$logs/build.log"
    return 1
  fi
  api=$(free_port)
  web=$(free_port)
  while [ "$web" = "$api" ]; do
    web=$(free_port)
  done
  node "$TASK_DIR/fake-apis.mjs" "$api" > "$logs/apis.log" 2>&1 &
  eval_servers+=("$!")
  (
    cd frontend/.next/standalone/frontend \
      && PORT=$web HOSTNAME=127.0.0.1 FACILITY_API_URL="http://127.0.0.1:$api" RESERVATION_API_URL="http://127.0.0.1:$api" \
        exec node server.js
  ) > "$logs/web.log" 2>&1 &
  eval_servers+=("$!")
  until curl -fsS --max-time 2 "http://127.0.0.1:$web/healthz" > /dev/null 2>&1; do
    tries=$((tries + 1))
    if [ "$tries" -ge 60 ]; then
      echo "the frontend did not answer within 30 seconds; the end of its log:"
      tail -n 3 "$logs/web.log"
      return 1
    fi
    sleep 0.5
  done
  if ! curl -fsS --max-time 60 "http://127.0.0.1:$web/rooms/$room?date=2026-11-20" > "$logs/page.html"; then
    echo "the page of the room did not answer with 200"
    return 1
  fi
  node "$TASK_DIR/page-text.mjs" < "$logs/page.html" > "$logs/page.txt"
}

# The page of the room shows the length of each reservation, each as a
# whole: not a part of "11 h", and not "2 h 0 min" or "0 h 45 min".
page_shows_the_lengths() {
  local status=0 text
  rm -rf "$logs"
  mkdir -p "$logs"
  trap stop_servers EXIT
  read_the_page || status=$?
  stop_servers
  trap - EXIT
  if [ "$status" -ne 0 ]; then
    return "$status"
  fi
  text=$(cat "$logs/page.txt")
  if ! grep -q -E '(^|[^0-9])1 h 30 min($|[^a-z])' <<< "$text" \
    || ! grep -q -E '(^|[^0-9])45 min($|[^a-z])' <<< "$text" \
    || grep -q -E '(^|[^0-9])0 h 45 min' <<< "$text" \
    || ! grep -q -E '(^|[^0-9])2 h($|[^0-9a-z ]| [^0-9])' <<< "$text"; then
    # The list streams in last, so the end of the text shows it.
    echo "the page does not show the lengths 1 h 30 min, 45 min, and 2 h; it ends with: ...${text: -240}"
    return 1
  fi
}

# The length comes from pure logic with a unit test (the guide, step 3): a
# test of src/ is new or changed.
unit_test_of_the_logic() {
  if ! changed_paths | grep -q -E '^frontend/src/.*\.test\.ts$'; then
    echo "no unit test under frontend/src/ is new or changed"
    return 1
  fi
}

# What a person sees is a scenario in a browser (the guide, step 7): a file
# of frontend/e2e/ is new or changed.
scenario_in_a_browser() {
  if ! changed_paths | grep -q '^frontend/e2e/'; then
    echo "no scenario in frontend/e2e/ is new or changed"
    return 1
  fi
}

grade_task() {
  criterion page-shows-the-lengths page_shows_the_lengths
  criterion unit-test-of-the-logic unit_test_of_the_logic
  criterion scenario-in-a-browser scenario_in_a_browser
}
