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
#             no .claude/ (settings, hooks, skills, subagents). Everything
#             else stays, the checks included: `make check` exists in both
#             arms, and only the harness arm is told to run it.
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
#   solution.patch   a change that passes every criterion: it proves that the
#                    task can be done and that the grader can tell
#
# Every task also gets the criteria that every change of this repository
# must meet: `make check` passes, and the change weakens no check (it touches
# no allowance or ignore file, no hook, no check, and adds no ignore comment).
# After a run of the bare arm, the files of the harness are put back before
# the grading, so both arms are graded as the same repository.
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

# The files of the harness, as the commit holds them.
harness_paths() {
  git ls-tree -r --name-only "$ref" | grep -E '(^|/)(AGENTS(\.ja)?|CLAUDE)\.md$|^\.claude/'
}

# What a change may not touch: the files through which a check is weakened
# instead of satisfied.
protected='^(\.claude/|(.*/)?(AGENTS(\.ja)?|CLAUDE)\.md$|docs/adr/template/|schema-changes\.allow$|\.trivyignore\.yaml$|\.i18nignore$|\.lintignore$|lefthook\.yml$|\.github/|Makefile$|make/|tools/|evals/|\.golangci\.yml$|\.vale\.ini$|\.textlintrc\.json$|\.squawk.*\.toml$|trivy\.yaml$|redocly\.yaml$|_typos\.toml$)'
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
  local dir="$1" paths index tree
  paths=$(harness_paths)
  # shellcheck disable=SC2086 # one argument for each path
  git archive --format=tar "$ref" -- $paths | tar -x -C "$dir"
  (
    cd "$dir"
    index="$PWD/.git/graded-index"
    GIT_INDEX_FILE="$index" git read-tree base
    # shellcheck disable=SC2086 # one argument for each path
    GIT_INDEX_FILE="$index" git add -- $paths
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

# The checks of the repository, without their time budget: a grader measures
# the change, not the machine. golangci-lint refuses to run twice at once on
# one machine, so a run that met another one is tried again.
checks_pass() {
  local out tries=0
  until out=$(CHECK_BUDGET=0 CHECK_BASE=graded make check 2>&1); do
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
# terminal, stopped after $timeout seconds.
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
      --output-format json --model "$model" --max-turns "$max_turns" \
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
      pkill -INT -P "$pid" 2> /dev/null || true
      sleep 15
      pkill -TERM -P "$pid" 2> /dev/null || true
    fi
  done
  wait "$pid" || true
}

run() {
  local task="$1" arm="$2" id dir out record started seconds
  known "$task"
  case "$arm" in
    harness | bare) ;;
    *) fail "the arm is harness or bare" ;;
  esac
  command -v "$claude" > /dev/null || fail "\"$claude\" is not a command; set EVAL_CLAUDE to the command of Claude Code"

  id="$(date -u +%Y%m%dT%H%M%SZ)-$task-$arm"
  dir="$work/$id"
  out="$work/$id.agent.json"
  criteria="$work/$id.criteria"
  mkdir -p "$work" "$results"

  echo "evals: $task, $arm: preparing $dir"
  prepare "$dir" "$arm"
  (cd "$dir" && make setup) > "$work/$id.setup.log" 2>&1 || fail "make setup failed in the copy; see $work/$id.setup.log"

  echo "evals: $task, $arm: the agent works (model $model, at most $max_turns turns and $timeout seconds)"
  started=$(date +%s)
  run_agent "$dir" "$task" "$out"
  seconds=$(($(date +%s) - started))

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
    --slurpfile agent <(jq -c . "$out" 2> /dev/null || echo null) \
    --rawfile criteria "$criteria" \
    --rawfile patch "$work/$id.patch" '
    ($agent[0] // {}) as $a
    | ($criteria | split("\n") | map(select(length > 0) | split("\t")
        | {name: .[1], pass: (.[0] == "pass"), detail: (.[2] // "")})) as $c
    | {
        task: $task, arm: $arm, model: $model, commit: $commit, started: $started, cli: $cli,
        seconds: $seconds,
        agent: {
          outcome: ($a.subtype // "no result"),
          turns: ($a.num_turns // null),
          cost_usd: ($a.total_cost_usd // null),
          output_tokens: ($a.usage.output_tokens // null),
          refused_commands: (($a.permission_denials // []) | length)
        },
        changed_lines: ($patch | split("\n") | map(select(test("^[+-][^+-]"))) | length),
        criteria: $c,
        passed: ($c | all(.pass))
      }' > "$record"
  jq -r '"evals: \(.task), \(.arm): \(if .passed then "passed" else "failed" end) in \(.seconds) s, \(.agent.turns // "?") turns",
    (.criteria[] | "  \(if .pass then "pass" else "FAIL" end)  \(.name)  \(.detail)")' "$record"
  echo "evals: the record is $record; the change of the agent is $work/$id.patch"

  if [ "${EVAL_KEEP:-}" != 1 ]; then
    rm -rf "${dir:?}"
  fi
}

# --- Proving the graders ---------------------------------------------------

# For each task: the repository as it is must fail a criterion of the task,
# and with the solution it must pass every criterion, the common ones
# included. One finding per line.
verify() {
  local task dir status=0 selected
  selected="$*"
  [ -n "$selected" ] || selected=$(list)
  mkdir -p "$work"
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

    if [ -f "$tasks/$task/solution.patch" ]; then
      if ! git -C "$dir" apply --whitespace=nowarn "$repo/$tasks/$task/solution.patch" 2> "$criteria.apply"; then
        echo "$tasks/$task/solution.patch:1: [evals] the solution does not apply to the repository any more: $(tail -n 1 "$criteria.apply")"
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
    rm -rf "${dir:?}" "$criteria" "$criteria.apply"
  done
  return "$status"
}

# --- The records -----------------------------------------------------------

report() {
  local files
  files=$(find "$results" -name '*.json' 2> /dev/null | sort)
  if [ -z "$files" ]; then
    echo "evals: no record in $results/ yet"
    return 0
  fi
  echo "| Task | Arm | Model | Runs | Passed | Criteria passed | Turns | Minutes | Refused commands |"
  echo "|---|---|---|---|---|---|---|---|---|"
  # shellcheck disable=SC2086 # one argument for each file
  jq --slurp --raw-output '
    def median: sort | if length == 0 then null else .[(length - 1) / 2 | floor] end;
    group_by([.task, .arm, .model])[]
    | "| \(.[0].task) | \(.[0].arm) | \(.[0].model) | \(length) | \(map(select(.passed)) | length) | "
      + "\(map(.criteria | map(select(.pass)) | length) | add) of \(map(.criteria | length) | add) | "
      + "\(map(.agent.turns // empty) | median // "?") | \(map(.seconds / 60 | floor) | median) | "
      + "\(map(.agent.refused_commands) | add) |"' $files
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
