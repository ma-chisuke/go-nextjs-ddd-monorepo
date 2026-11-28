#!/usr/bin/env bash
# The evaluation of the agent harness (TPL-0020): a coding agent does a task
# of this repository twice, once with the harness and once without it, and a
# grader says what each result is worth. An instruction is added to AGENTS.md
# only after a run here shows a failure that no check catches.
#
# Usage: run.sh list                  the tasks
#        run.sh run <task> <arm>      let the agent do the task, grade the
#                                     result, and write a record to
#                                     evals/results/
#        run.sh verify [<task>...]    prove the graders without an agent: the
#                                     repository as it is fails the task, and
#                                     with the solution of the task it passes
#        run.sh report                a table of the records in evals/results/
#
# The arms:
#   harness   the repository as it is
#   bare      the repository without the harness: no AGENTS.md, no CLAUDE.md,
#             no .claude/ (settings, hooks, skills, subagents), and neither
#             the tests of the hooks nor the line of the README that lists
#             the instruction files. Everything else stays, the checks
#             included: `make check` exists and passes in both arms, and
#             only the harness arm is told to run it.
#
# A run copies the files of a commit (EVAL_REF, default HEAD) into a
# directory outside this checkout and makes a Git repository of its own there,
# so the agent cannot touch this repository, and its parent directories hold
# no instruction file. `make setup` then prepares the copy as a new clone is
# prepared.
#
# The agent is Claude Code, without a terminal (`claude -p`). Both arms get
# the same model, the same limits, and the same permissions: the tools in
# evals/allowed-tools.txt, and no question to a person, so a command outside
# that list is refused and the refusal is counted. Settings and memory of the
# user are left out (`--setting-sources project,local`), so a run does not
# depend on the machine of whoever starts it.
#
#   EVAL_CLAUDE      the command of Claude Code (default: claude)
#   EVAL_MODEL       the model (default: sonnet)
#   EVAL_MAX_TURNS   the limit of turns (default: 150)
#   EVAL_TIMEOUT     the limit of time, in seconds (default: 2700)
#   EVAL_REF         the commit to start from (default: HEAD)
#   EVAL_KEEP        1 keeps the copy after the run, for a look at it
#
# A task is a directory evals/tasks/<task>/ with
#   task.txt         what the agent is asked to do, as a person would ask
#   grade.sh         the criteria of the task: a function `grade_task` that
#                    calls `criterion <name> <command>...` for each one. A
#                    criterion passes when its command succeeds in the copy.
#                    The helpers under "What a grader may use" below give it
#                    a database, a port, or a test run of its own.
#   solution.patch   a change that passes every criterion: it proves that the
#                    task can be done and that the grader can tell
#
# A grader needs no Docker of its own and no network. Some use the local
# PostgreSQL, which must run (`make db-server`), with a database of their own.
#
# Every task also gets the criteria that every change of this repository
# must meet: `make check` passes, and the change weakens no check (it touches
# no allowance or ignore file, no hook, no check, and adds no ignore comment).
# After a run of the bare arm, the files of the harness are put back before
# the grading, so both arms are graded with the same checks.
set -euo pipefail

# Nothing of the make that called this script may reach the copies: its
# variables would override those of the make files there, and its scope
# would narrow the checks.
unset MAKEFLAGS MFLAGS MAKELEVEL MAKEOVERRIDES CHECK_SCOPE CHECK_FILES CHECK_BASE CHECK_STATE_DIR VERBOSE
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX

cd "$(git rev-parse --show-toplevel)"
repo=$PWD
tasks=evals/tasks
results=evals/results

claude="${EVAL_CLAUDE:-claude}"
model="${EVAL_MODEL:-sonnet}"
max_turns="${EVAL_MAX_TURNS:-150}"
timeout="${EVAL_TIMEOUT:-2700}"
ref="${EVAL_REF:-HEAD}"

work="${EVAL_WORK:-${TMPDIR:-/tmp}}"
work="${work%/}/evals-$(basename "$repo")"

fail() {
  echo "evals: $*" >&2
  exit 2
}

list() {
  local dir
  for dir in "$tasks"/*/; do
    [ -f "${dir}task.txt" ] || continue
    dir=${dir%/}
    printf '%s\n' "${dir##*/}"
  done
}

known() {
  # grep reads the whole list: with -q it would stop at the first match,
  # and the list, still writing, would fail the pipeline.
  list | grep -x -- "${1:-}" > /dev/null || fail "no task is named \"${1:-}\"; the tasks are: $(list | tr '\n' ' ')"
}

# The files of the harness, as the commit holds them: the instruction files,
# .claude/, and the check that tests the hooks, which has nothing to test
# without them.
harness_paths() {
  git ls-tree -r --name-only "$ref" | grep -E '(^|/)(AGENTS(\.ja)?|CLAUDE)\.md$|^\.claude/|^make/harness\.mk$|^tools/tests/hooks_test\.sh$'
}

# unlist_harness <directory>: the README lists the instruction files. In the
# bare arm that line would link to nothing, and lint-links would fail whatever
# the agent did. The line goes in both languages, and the marker of the
# translation follows its source (tools/internal/i18n). The line does not come
# back before the grading: no check reads it.
#
# ignore-comments.allow names ignore comments in files of the harness, which
# are gone too: its lines for those files go, and the file comes back with
# the harness (restore_harness).
unlist_harness() {
  local dir="$1" file hash
  if [ -f "$dir/ignore-comments.allow" ]; then
    grep -v -F -f <(harness_paths | sed 's/$/ | /') "$dir/ignore-comments.allow" > "$dir/ignore-comments.allow.new"
    mv "$dir/ignore-comments.allow.new" "$dir/ignore-comments.allow"
  fi
  for file in README.md README.ja.md; do
    grep -v -E '^- \[AGENTS(\.ja)?\.md\]\(AGENTS(\.ja)?\.md\)' "$dir/$file" > "$dir/$file.new"
    mv "$dir/$file.new" "$dir/$file"
  done
  hash=$(shasum -a 256 "$dir/README.md" | cut -d ' ' -f 1)
  sed -e "s/^\(<!-- i18n-source: README\.md sha256:\)[0-9a-f]*/\1$hash/" "$dir/README.ja.md" > "$dir/README.ja.md.new"
  mv "$dir/README.ja.md.new" "$dir/README.ja.md"
}

# What a change may not touch: the files through which a check is weakened
# instead of satisfied.
protected='^(\.claude/|(.*/)?(AGENTS(\.ja)?|CLAUDE)\.md$|docs/adr/template/|schema-changes\.allow$|equivalent-mutants\.allow$|ignore-comments\.allow$|\.trivyignore\.yaml$|\.i18nignore$|\.lintignore$|lefthook\.yml$|\.github/|Makefile$|make/|tools/|evals/|\.golangci\.yml$|\.vale\.ini$|\.textlintrc\.json$|\.squawk.*\.toml$|trivy\.yaml$|redocly\.yaml$|_typos\.toml$)'
ignore_comment='nolint|eslint-disable|@ts-ignore|@ts-expect-error|@ts-nocheck|trivy:ignore|shellcheck disable=|vale off|textlint-disable|markdownlint-disable|squawk-ignore|tflint-ignore|#nosec|prettier-ignore'

# prepare <directory> <arm>: the files of the commit in a repository of their
# own, with one commit, `base`.
prepare() {
  local dir="$1" arm="$2" path
  mkdir -p "$dir"
  git archive --format=tar "$ref" | tar -x -C "$dir"
  # The tasks, their graders, and their solutions stay here: an agent that
  # found them in its copy would not have to do the task.
  rm -rf "${dir:?}/evals"
  if [ "$arm" = bare ]; then
    harness_paths | while IFS= read -r path; do
      rm -f "${dir:?}/$path"
    done
    rm -rf "${dir:?}/.claude"
    unlist_harness "$dir"
  fi
  (
    cd "$dir"
    git init --quiet --initial-branch=main
    git config user.name "eval"
    git config user.email "eval@example.invalid"
    printf 'node_modules\n.venv\n' >> .git/info/exclude
    git add --all
    git commit --quiet --message "The repository at $(git -C "$repo" rev-parse --short "$ref")"
    # `base` is what the agent starts from, and `graded` what its change is
    # compared with. They differ after a run of the bare arm (restore_harness).
    git tag base
    if [ "$arm" = harness ]; then
      git tag graded
    fi
  )
}

# The dependencies of this checkout, linked into a copy that only a grader
# reads. A run of an agent installs its own (`make setup`).
link_dependencies() {
  local dir="$1" path
  for path in node_modules .venv frontend/node_modules; do
    if [ -e "$repo/$path" ] && [ ! -e "$dir/$path" ]; then
      ln -s "$repo/$path" "$dir/$path"
    fi
  done
}

# restore_harness <directory>: after a run of the bare arm, the files of the
# harness come back, and the tag `graded` names the commit `base` with those
# files: compared with it, the change of the agent is its own work alone. The
# copy held no trace of the harness while the agent worked.
restore_harness() {
  local dir="$1" listed path index tree
  local paths=()
  listed=$(harness_paths)
  while IFS= read -r path; do
    paths+=("$path")
  done <<< "$listed"
  if git cat-file -e "$ref:ignore-comments.allow" 2> /dev/null; then
    paths+=(ignore-comments.allow)
  fi
  git archive --format=tar "$ref" -- "${paths[@]}" | tar -x -C "$dir"
  (
    cd "$dir"
    index="$PWD/.git/graded-index"
    GIT_INDEX_FILE="$index" git read-tree base
    GIT_INDEX_FILE="$index" git add -- "${paths[@]}"
    tree=$(GIT_INDEX_FILE="$index" git write-tree)
    git tag graded "$(git commit-tree "$tree" -p base -m "The repository with its harness")"
    rm -f "$index"
  )
}

# --- Grading ---------------------------------------------------------------

# criterion <name> <command>...: one line in $criteria, "pass" or "fail", the
# name, and for a failure the end of what the command printed.
criterion() {
  local name="$1" out
  shift
  if out=$("$@" 2>&1); then
    printf 'pass\t%s\t\n' "$name" >> "$criteria"
  else
    printf 'fail\t%s\t%s\n' "$name" "$(printf '%s' "$out" | tail -n 4 | tr '\n\t' '  ' | cut -c 1-500)" >> "$criteria"
  fi
}

# checks_pass [<base>]: the checks of the repository, compared with the commit
# <base> (default: graded), without their time budget: a grader measures the
# change, not the machine. A copy from before golangci-lint was allowed to
# run twice at once on one machine (EVAL_REF) can still meet another run,
# and is tried again.
checks_pass() {
  local base="${1:-graded}" out tries=0
  until out=$(CHECK_BUDGET=0 CHECK_BASE="$base" make check 2>&1); do
    tries=$((tries + 1))
    if [ "$tries" -ge 3 ] || ! grep -q 'parallel golangci-lint is running' <<< "$out"; then
      # The checks that failed, by name, then the end of what they printed.
      printf 'failed: %s\n' "$(printf '%s\n' "$out" | sed -n 's/^FAIL  *\([^ ]*\).*/\1/p' | tr '\n' ' ')"
      printf '%s\n' "$out" | grep -v -e '^ok ' -e '^FAIL ' | tail -n 3
      return 1
    fi
    sleep 20
  done
}

# The change, staged: what differs from `graded`, new files included.
changed_paths() {
  git add --all > /dev/null 2>&1
  git diff --cached --name-only graded
}

rules_kept() {
  local touched added
  touched=$(changed_paths | grep -E "$protected" || true)
  if [ -n "$touched" ]; then
    echo "the change touches what a change may not touch: $(printf '%s' "$touched" | tr '\n' ' ')"
    return 1
  fi
  added=$(git diff --cached graded | grep '^+' | grep -v '^+++' | grep -E -o "$ignore_comment" | sort -u || true)
  if [ -n "$added" ]; then
    echo "the change adds an ignore comment: $(printf '%s' "$added" | tr '\n' ' ')"
    return 1
  fi
}

# What a grader may use, besides `criterion` and `changed_paths`: what the
# copy does not hold by itself.

# go_test <argument>...: `go test` in backend/, which prints only the tests
# that failed and their first messages: a criterion keeps the end of what
# its command printed, and the end of `go test` names only the packages.
go_test() {
  local out
  if ! out=$(cd backend && go test "$@" 2>&1); then
    printf '%s\n' "$out" | grep -v -E '^(ok|\?|PASS|FAIL$|=== )' | head -n 4
    return 1
  fi
}

# integration_tests <directory>: the tests of the files behind the build tag
# `integration` under backend/<directory>, and none of the others, which need
# no database. They need PGDATABASE (with_database). The tests of a file that
# needs AWS Emulator (awstest) are left out: no grader starts it.
integration_tests() {
  local names
  names=$(cd backend && grep -rl --include='*_test.go' '^//go:build integration' "$1" | xargs grep -L awstest \
    | xargs sed -n 's/^func \(Test[A-Za-z0-9_]*\)(.*/\1/p' | paste -s -d '|' -)
  if [ -z "$names" ]; then
    echo "backend/$1 holds no integration test"
    return 1
  fi
  go_test -count=1 -tags integration -run "^($names)\$" "./$1/..."
}

# with_database <command>...: runs the command with PGDATABASE naming a
# database of the copy's own on the local PostgreSQL, made anew with every
# schema.sql of the copy applied, and drops the database afterwards. The name
# comes from the directory of the copy, so two copies never share one, and
# none of them touches the databases of the local system or of the tests,
# where another checkout may apply another schema. The server must run:
# make test-evals waits for db-server.
with_database() {
  local database status=0
  database="evals_$(printf '%s' "$PWD" | cksum | cut -d ' ' -f 1)"
  drop_database "$database" || return 1
  # The trap runs when the script ends, where the local variable is gone:
  # it reads the name from a variable of the script.
  scratch_database=$database
  trap 'drop_database "$scratch_database"' EXIT
  if PGDATABASE=$database tools/scripts/db.sh init > /dev/null; then
    (
      export PGDATABASE=$database
      "$@"
    ) || status=$?
  else
    status=1
  fi
  drop_database "$database"
  trap - EXIT
  return "$status"
}

drop_database() {
  docker compose exec -T -e PGOPTIONS='-c client_min_messages=warning' postgres \
    psql --username postgres --dbname postgres --quiet --set ON_ERROR_STOP=1 \
    --command "DROP DATABASE IF EXISTS $1 WITH (FORCE)" > /dev/null
}

# free_port: a port of 127.0.0.1 on which nothing listens, from 20000 to
# 32767, as tools/scripts/api-fuzz.sh chooses one.
free_port() {
  local port
  while :; do
    port=$((20000 + RANDOM % 12768))
    if ! (exec 3<> "/dev/tcp/127.0.0.1/$port") 2> /dev/null; then
      echo "$port"
      return
    fi
  done
}

# own_dependencies: the packages of Node installed in the copy itself, for a
# grader that builds the frontend: Next.js does not build from a node_modules
# that links out of the copy, as link_dependencies makes it for `verify`.
# pnpm installs them from its store, without the network. The copy of a run
# has its own (make setup).
own_dependencies() {
  if [ -L node_modules ] || [ -L frontend/node_modules ]; then
    rm -f node_modules frontend/node_modules
    pnpm install --frozen-lockfile --offline > /dev/null
  fi
}

# grade <task> <directory> <which>: writes $criteria. `which` is "task" for
# the criteria of the task alone, or "all".
grade() {
  local task="$1" dir="$2" which="$3"
  : > "$criteria"
  (
    cd "$dir"
    TASK_DIR="$repo/$tasks/$task"
    export TASK_DIR
    # shellcheck source=/dev/null
    . "$TASK_DIR/grade.sh"
    grade_task
    if [ "$which" = all ]; then
      criterion rules-kept rules_kept
      criterion checks-pass checks_pass
    fi
  )
}

failures() {
  grep -c '^fail' "$criteria" || true
}

# --- A run of the agent ----------------------------------------------------

# run_agent <directory> <task> <output>: Claude Code on the task, without a
# terminal, stopped after $timeout seconds. The output is the stream of the
# run, one JSON object per line. A run that is stopped leaves <output>.stopped.
run_agent() {
  local dir="$1" task="$2" out="$3" pid waited=0 tool
  local -a allowed=()
  while IFS= read -r tool; do
    case "$tool" in
      '' | '#'*) ;;
      *) allowed+=("$tool") ;;
    esac
  done < evals/allowed-tools.txt
  # `make check` has a time budget, which says something about the machine,
  # not about the change: a run measures the agent, so the budget is off for
  # what the agent starts, in both arms.
  (
    cd "$dir"
    CHECK_BUDGET=0 CLAUDE_CODE_DISABLE_AUTO_MEMORY=1 "$claude" -p "Do the task that follows, in this repository." \
      --output-format stream-json --verbose --model "$model" --max-turns "$max_turns" \
      --setting-sources project,local --permission-mode acceptEdits \
      --no-session-persistence --allowedTools "${allowed[@]}" \
      < "$repo/$tasks/$task/task.txt" > "$out" 2> "$out.err"
  ) &
  pid=$!
  while kill -0 "$pid" 2> /dev/null; do
    sleep 5
    waited=$((waited + 5))
    if [ "$waited" -ge "$timeout" ]; then
      echo "evals: the agent ran for $timeout seconds; stopping it" >&2
      : > "$out.stopped"
      pkill -INT -P "$pid" 2> /dev/null || true
      sleep 15
      pkill -TERM -P "$pid" 2> /dev/null || true
    fi
  done
  wait "$pid" || true
}

# results_of <stream>: the results of a run, one JSON object per line. Claude
# Code ends a stretch of work with a result, and a run has one stretch more
# for each time that a command in the background wakes the agent. A line that
# a stopped run did not finish is left out.
results_of() {
  jq --raw-input --compact-output 'fromjson? | select(type == "object" and .type == "result")' "$1"
}

# summary_of <results> <stopped>: what the record says about the agent. The
# turns and the refused commands of a result are those of its stretch, so the
# record adds them up. The cost and the usage of each model are those of the
# whole run in every result, so the record takes them from the last one.
summary_of() {
  jq --slurp --compact-output --arg stopped "$2" '
    (last // {}) as $a
    | {
        outcome: (if $stopped == "true" then "stopped at the limit of time" else ($a.subtype // "no result") end),
        turns: (if length == 0 then null else (map(.num_turns // 0) | add) end),
        cost_usd: ($a.total_cost_usd // null),
        output_tokens: ([$a.modelUsage[]?.outputTokens] | add),
        refused_commands: (if length == 0 then null else (map((.permission_denials // []) | length) | add) end)
      }' "$1"
}

run() {
  local task="$1" arm="$2" id dir stream out record started seconds stopped=false
  known "$task"
  case "$arm" in
    harness | bare) ;;
    *) fail "the arm is harness or bare" ;;
  esac
  command -v "$claude" > /dev/null || fail "\"$claude\" is not a command; set EVAL_CLAUDE to the command of Claude Code"

  id="$(date -u +%Y%m%dT%H%M%SZ)-$task-$arm"
  dir="$work/$id"
  stream="$work/$id.agent.jsonl"
  out="$work/$id.agent.json"
  criteria="$work/$id.criteria"
  mkdir -p "$work" "$results"

  echo "evals: $task, $arm: preparing $dir"
  prepare "$dir" "$arm"
  (cd "$dir" && make setup) > "$work/$id.setup.log" 2>&1 || fail "make setup failed in the copy; see $work/$id.setup.log"

  echo "evals: $task, $arm: the agent works (model $model, at most $max_turns turns and $timeout seconds)"
  started=$(date +%s)
  run_agent "$dir" "$task" "$stream"
  seconds=$(($(date +%s) - started))
  results_of "$stream" > "$out"
  if [ -e "$stream.stopped" ]; then
    stopped=true
  fi

  # A run that something outside the run ended measures nothing about the
  # agent: the usage limit of the account, or an error of the API. Claude Code
  # then reports an error that is none of the limits of a run (the turns), and
  # no record is written. A run that the watchdog stopped has no result, and
  # is recorded as it is.
  if tail -n 1 "$out" | jq -e '.terminal_reason == "api_error" or (.is_error == true and .subtype == "success")' > /dev/null 2>&1; then
    fail "the agent was stopped from outside, and no record is written: $(tail -n 1 "$out" | jq -r '.result | tostring | .[0:200]')"
  fi

  (cd "$dir" && git add --all > /dev/null 2>&1 && git diff --cached base) > "$work/$id.patch"
  if [ "$arm" = bare ]; then
    restore_harness "$dir"
  fi
  echo "evals: $task, $arm: grading"
  grade "$task" "$dir" all

  record="$results/$id.json"
  jq --null-input \
    --arg task "$task" --arg arm "$arm" --arg model "$model" \
    --arg commit "$(git rev-parse "$ref")" --arg started "${id%%-*}" \
    --arg cli "$("$claude" --version 2> /dev/null | head -n 1)" \
    --argjson seconds "$seconds" \
    --argjson agent "$(summary_of "$out" "$stopped")" \
    --rawfile criteria "$criteria" \
    --rawfile patch "$work/$id.patch" '
    ($criteria | split("\n") | map(select(length > 0) | split("\t")
        | {name: .[1], pass: (.[0] == "pass"), detail: (.[2] // "")})) as $c
    | {
        task: $task, arm: $arm, model: $model, commit: $commit, started: $started, cli: $cli,
        seconds: $seconds,
        agent: $agent,
        changed_lines: ($patch | split("\n") | map(select(test("^[+-][^+-]"))) | length),
        criteria: $c,
        passed: ($c | all(.pass))
      }' > "$record"
  jq -r '"evals: \(.task), \(.arm): \(if .passed then "passed" else "failed" end) in \(.seconds) s, \(.agent.output_tokens // "?") output tokens",
    (.criteria[] | "  \(if .pass then "pass" else "FAIL" end)  \(.name)  \(.detail)")' "$record"
  echo "evals: the record is $record; the change of the agent is $work/$id.patch, and its results are $out"

  if [ "${EVAL_KEEP:-}" != 1 ]; then
    rm -rf "${dir:?}"
  fi
}

# --- Proving the graders ---------------------------------------------------

# For each task: the repository as it is must fail a criterion of the task,
# and with the solution it must pass every criterion, the common ones
# included. One finding per line.
# verify_bare: the bare arm, without an agent. `make check` passes in a copy
# without the harness, so a check that fails there after a run is the work of
# the agent, and it passes again once the harness is back.
verify_bare() {
  local dir="$work/verify-bare-$$" out status=0
  prepare "$dir" bare
  link_dependencies "$dir"
  if ! out=$(cd "$dir" && checks_pass base); then
    echo "evals/run.sh:1: [evals] make check fails in the bare arm before an agent worked: $(printf '%s' "$out" | tr '\n' ' ' | cut -c 1-300)"
    status=1
  fi
  restore_harness "$dir"
  if ! out=$(cd "$dir" && checks_pass); then
    echo "evals/run.sh:1: [evals] make check fails in the bare arm after the harness came back: $(printf '%s' "$out" | tr '\n' ' ' | cut -c 1-300)"
    status=1
  fi
  rm -rf "${dir:?}"
  return "$status"
}

# The summary of a run adds up its stretches: proven on a stream of two
# stretches, with a line that is no result and a line that was cut off.
verify_summary() {
  local stream="$work/verify-summary-$$.jsonl" got want
  {
    echo '{"type":"system","subtype":"init"}'
    echo '{"type":"result","subtype":"success","num_turns":3,"permission_denials":[{"tool_name":"Bash"}],"total_cost_usd":0.1,"modelUsage":{"a":{"outputTokens":10},"b":{"outputTokens":200}}}'
    echo '{"type":"result","subtype":"success","num_turns":2,"permission_denials":[],"total_cost_usd":0.2,"modelUsage":{"a":{"outputTokens":10},"b":{"outputTokens":350}}}'
    printf '%s' '{"type":"result","subtype":"succ'
  } > "$stream"
  results_of "$stream" > "$stream.results"
  got=$(summary_of "$stream.results" false)
  want='{"outcome":"success","turns":5,"cost_usd":0.2,"output_tokens":360,"refused_commands":1}'
  rm -f "$stream" "$stream.results"
  if [ "$got" != "$want" ]; then
    echo "evals/run.sh:1: [evals] the summary of a run of two stretches is $got, and must be $want"
    return 1
  fi
}

verify() {
  local task dir status=0 selected
  selected="$*"
  mkdir -p "$work"
  # Without a named task, the arms are proven too.
  if [ -z "$selected" ]; then
    selected=$(list)
    verify_bare || status=1
    verify_summary || status=1
  fi
  for task in $selected; do
    known "$task"
    dir="$work/verify-$task-$$"
    criteria="$work/verify-$task-$$.criteria"
    prepare "$dir" harness
    link_dependencies "$dir"

    grade "$task" "$dir" task
    if [ "$(failures)" -eq 0 ]; then
      echo "$tasks/$task/grade.sh:1: [evals] every criterion of the task passes before anyone did the task: the grader cannot tell"
      status=1
    fi

    # A solution holds no generated code (*.gen.go, *.gen.ts): `make gen`
    # writes it after the solution, as whoever does the task runs it. A
    # change to a document or to a generator would otherwise break every
    # solution that holds the code generated from it.
    if [ -f "$tasks/$task/solution.patch" ]; then
      if ! git -C "$dir" apply --whitespace=nowarn "$repo/$tasks/$task/solution.patch" 2> "$criteria.apply"; then
        echo "$tasks/$task/solution.patch:1: [evals] the solution does not apply to the repository any more: $(tail -n 1 "$criteria.apply")"
        status=1
      elif ! (cd "$dir" && make gen) > "$criteria.gen" 2>&1; then
        echo "$tasks/$task/solution.patch:1: [evals] make gen fails after the solution: $(tail -n 1 "$criteria.gen")"
        status=1
      else
        grade "$task" "$dir" all
        if [ "$(failures)" -ne 0 ]; then
          grep '^fail' "$criteria" | while IFS=$'\t' read -r _ name detail; do
            echo "$tasks/$task/solution.patch:1: [evals] the solution fails the criterion $name: $detail"
          done
          status=1
        fi
      fi
    else
      echo "$tasks/$task/grade.sh:1: [evals] the task has no solution.patch: nothing proves that it can be done"
      status=1
    fi
    rm -rf "${dir:?}" "$criteria" "$criteria.apply" "$criteria.gen"
  done
  return "$status"
}

# --- The records -----------------------------------------------------------

report() {
  local listed file
  local files=()
  listed=$(find "$results" -name '*.json' 2> /dev/null | sort)
  while IFS= read -r file; do
    [ -z "$file" ] || files+=("$file")
  done <<< "$listed"
  if [ "${#files[@]}" -eq 0 ]; then
    echo "evals: no record in $results/ yet"
    return 0
  fi
  echo "| Task | Arm | Model | Runs | Passed | Criteria passed | Output tokens | Minutes | Refused commands |"
  echo "|---|---|---|---|---|---|---|---|---|"
  jq --slurp --raw-output '
    def median: sort | if length == 0 then null else .[(length - 1) / 2 | floor] end;
    group_by([.task, .arm, .model])[]
    | "| \(.[0].task) | \(.[0].arm) | \(.[0].model) | \(length) | \(map(select(.passed)) | length) | "
      + "\(map(.criteria | map(select(.pass)) | length) | add) of \(map(.criteria | length) | add) | "
      + "\(map(.agent.output_tokens // empty) | median // "?") | \(map(.seconds / 60 | floor) | median) | "
      + "\(map(.agent.refused_commands // empty) | add // "?")"
      + (map(select(.agent.refused_commands == null)) | length
          | if . > 0 then ", and \(.) not counted |" else " |" end)' "${files[@]}"
}

case "${1:-}" in
  list)
    list
    ;;
  run)
    run "${2:-}" "${3:-}"
    ;;
  verify)
    shift
    verify "$@"
    ;;
  report)
    report
    ;;
  *)
    echo "usage: run.sh list | run <task> <harness|bare> | verify [<task>...] | report" >&2
    exit 2
    ;;
esac
