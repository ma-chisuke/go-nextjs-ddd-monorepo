#!/usr/bin/env bash
# Mutation testing (docs/guides/mutation-tests.md). A tool changes the code in
# small ways, one change (a mutant) at a time, and runs the tests against each
# mutant. A mutant that every test passes shows a rule that the tests do not
# hold, and fails the check. gomutants does this for Go packages, and
# StrykerJS for the modules of the frontend that have a unit test next to
# them (frontend/stryker.config.mjs).
#
# Usage:
#   mutation.sh go <package directory>...   every mutant of the packages
#   mutation.sh go-mutant <mutant>          one mutant, named as a finding names it
#   mutation.sh frontend [<module>...]      every mutant of the frontend, or of these modules
#   mutation.sh frontend-mutant <mutant>    one mutant of the frontend
#
# Each mutant runs only the tests that run its code: in Go, the tests of its
# own package; in the frontend, the unit tests that reach its line. So the
# tests of a package or a module must hold its rules themselves.
# equivalent-mutants.allow names the mutants that a person showed to be
# equivalent: changes that no test can tell apart from the code. A line of it
# belongs to the frontend when its mutant names a JavaScript or TypeScript
# file, and to Go otherwise. See tools/gomutants/findings.jq and
# tools/stryker/findings.jq.
#
# MUTATION_WORKERS sets how many mutants run at once; by default, half of the
# processors, so that the other checks keep the rest.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
root=$PWD
allow_file=equivalent-mutants.allow
report_dir=.cache/mutation

workers=${MUTATION_WORKERS:-$(($(getconf _NPROCESSORS_ONLN) / 2))}
if [ "$workers" -lt 1 ]; then
  workers=1
fi

# A test that hangs on a mutant has noticed it. The floor of the time that a
# mutant may take is high, so that a busy machine, which starts a fresh test
# binary slowly, does not count a slow start as a hang.
gomutants=(go tool -modfile="$root/tools/go.mod" gomutants
  --cache=off --timeout-min=30s --detect-equivalent --workers="$workers")

# StrykerJS runs in frontend/. It exits with an error only when it cannot run
# the tests at all; the mutants that survive are findings of
# tools/stryker/findings.jq.
stryker=("$root/frontend/node_modules/.bin/stryker" run --concurrency "$workers")

# Whether a file, such as the file of a mutant, is JavaScript or TypeScript.
is_script() {
  case "$1" in
    *.js | *.jsx | *.mjs | *.cjs | *.ts | *.tsx | *.mts | *.cts) return 0 ;;
    *) return 1 ;;
  esac
}

# The directory of the Go module that holds a path, relative to the root.
module_of() {
  local dir=$1
  until [ -f "$dir/go.mod" ]; do
    if [ "$dir" = . ] || [ "$dir" = / ]; then
      echo "mutation.sh: no go.mod above $1" >&2
      return 1
    fi
    dir=$(dirname "$dir")
  done
  echo "$dir"
}

run_go() {
  local status=0 module report log line id file
  local -a modules=()
  for pkg in "$@"; do
    module=$(module_of "$pkg")
    case " ${modules[*]-} " in
      *" $module "*) ;;
      *) modules+=("$module") ;;
    esac
  done

  mkdir -p "$report_dir"
  for module in "${modules[@]}"; do
    local -a targets=()
    for pkg in "$@"; do
      if [ "$(module_of "$pkg")" = "$module" ]; then
        targets+=("./${pkg#"$module"/}")
      fi
    done
    report="$root/$report_dir/${module//\//-}.json"
    log="$root/$report_dir/${module//\//-}.log"
    rm -f "$report"
    if ! (cd "$module" && "${gomutants[@]}" --quiet --output="$report" "${targets[@]}") > "$log" 2>&1; then
      echo "$module: gomutants failed; its output follows. Fix: make the tests of ${targets[*]} pass with \`make test-go\` first."
      sed 's/^/    /' "$log"
      status=1
      continue
    fi
    jq -r --arg module "$module/" --arg packages "$(printf '%s\n' "$@")" \
      --rawfile allow <(cat "$allow_file" 2> /dev/null || true) --arg allowfile "$allow_file" \
      -f tools/gomutants/findings.jq "$report" > "$log.findings"
    if [ -s "$log.findings" ]; then
      LC_ALL=C sort -t: -k1,1 -k2,2n -k3 "$log.findings"
      status=1
    fi
  done

  # An allowance that names a file that is gone, or that is not Go, outside
  # the packages of the run; findings.jq reports those inside them. The lines
  # of the frontend belong to run_frontend.
  if [ -f "$allow_file" ]; then
    local n=0
    while IFS= read -r line; do
      n=$((n + 1))
      case "$line" in
        '' | '#'*) continue ;;
      esac
      id=${line%% | *}
      file=${id%%:*}
      if is_script "$file"; then
        continue
      fi
      case " $* " in
        *" $(dirname "$file") "*) continue ;;
      esac
      if [[ $file != *.go ]]; then
        echo "$allow_file:$n: [mutation/allowance-format] $file is neither a Go file nor a JavaScript or TypeScript one, so no mutant is $id. Fix: write the mutant as the finding names it (docs/guides/mutation-tests.md). See TPL-0023."
        status=1
      elif [ ! -f "$file" ]; then
        echo "$allow_file:$n: [mutation/unused-allowance] $file does not exist, so no mutant is $id. Fix: remove the line (docs/guides/mutation-tests.md). See TPL-0023."
        status=1
      fi
    done < "$allow_file"
  fi
  return "$status"
}

# One mutant, with the verdict of gomutants: exit 0 when a test kills it, 10
# when it lives, 1 when no test runs it.
run_go_mutant() {
  local id=$1 file module
  if [ -z "$id" ]; then
    echo "usage: make mutant ID='<mutant>', with the mutant as a finding of test-mutation names it" >&2
    return 2
  fi
  file=${id%%:*}
  module=$(module_of "$(dirname "$file")")
  mkdir -p "$report_dir"
  cd "$module"
  "${gomutants[@]}" --verbose --threshold-efficacy=100 --output="$root/$report_dir/mutant.json" \
    --run-mutant-id="${id#"$module"/}" "./$(dirname "${file#"$module"/}")"
}

# The modules of the frontend that frontend/stryker.config.mjs names, or only
# the modules given, as paths from the root such as frontend/src/lib/input.ts.
# A run of some modules leaves out the allowances of the other modules.
run_frontend() {
  local status=0 full=true mutate='' module
  local report=$report_dir/frontend.json log=$report_dir/frontend.log
  local -a narrow=()
  for module in "$@"; do
    mutate="${mutate:+$mutate,}${module#frontend/}"
    full=false
  done
  if [ -n "$mutate" ]; then
    narrow=(--mutate "$mutate")
  fi

  mkdir -p "$report_dir"
  rm -f "$report"
  if ! (cd frontend && STRYKER_REPORT="$root/$report" "${stryker[@]}" ${narrow[@]+"${narrow[@]}"}) > "$log" 2>&1; then
    echo "frontend: StrykerJS failed; its output follows. Fix: make the unit tests pass with \`make test-frontend\` first."
    sed 's/^/    /' "$log"
    return 1
  fi
  jq -r -L tools/stryker --arg dir frontend/ --arg full "$full" \
    --rawfile allow <(cat "$allow_file" 2> /dev/null || true) --arg allowfile "$allow_file" \
    -f tools/stryker/findings.jq "$report" > "$log.findings"
  if [ -s "$log.findings" ]; then
    LC_ALL=C sort -t: -k1,1 -k2,2n -k3 "$log.findings"
    status=1
  fi
  return "$status"
}

# One mutant of the frontend, with its verdict: exit 0 when a test kills it,
# 10 when it survives, 1 when no test runs it. StrykerJS cannot run a mutant
# by its name, so this runs every mutant of the declaration that holds it, and
# reads the one.
run_frontend_mutant() {
  local id=$1 file declaration range verdict
  local report=$report_dir/frontend-mutant.json log=$report_dir/frontend-mutant.log
  if [ -z "$id" ]; then
    echo "usage: make mutant-frontend ID='<mutant>', with the mutant as a finding of test-mutation-frontend names it" >&2
    return 2
  fi
  file=${id%%:*}
  declaration=${id#*:}
  declaration=${declaration%%:*}
  if [ ! -f "$file" ]; then
    echo "mutation.sh: $file does not exist, so no mutant is $id" >&2
    return 2
  fi
  range=$(jq -Rrs -L tools/stryker --arg name "$declaration" 'include "mutants"; lines_of($name)' "$file")
  if [ -z "$range" ]; then
    echo "mutation.sh: $file declares no $declaration at the start of a line, so no mutant is $id" >&2
    return 2
  fi

  mkdir -p "$report_dir"
  rm -f "$report"
  if ! (cd frontend && STRYKER_REPORT="$root/$report" "${stryker[@]}" --mutate "${file#frontend/}:$range") > "$log" 2>&1; then
    echo "frontend: StrykerJS failed; its output follows."
    sed 's/^/    /' "$log"
    return 1
  fi
  verdict=$(jq -r -L tools/stryker --arg id "$id" 'include "mutants";
    [mutants("frontend/")[] | select(.id == $id)] | first
    | if . == null then "MISSING"
      else "\(.status)\t\(.file):\(.line): `\(.original)` changed to `\(.replacement)` (\(.mutatorName))\(
        if .statusReason then ": \(.statusReason | squash)" else "" end)" end' "$report")
  case "${verdict%%$'\t'*}" in
    Killed | Timeout | RuntimeError)
      echo "KILLED ${verdict#*$'\t'}"
      ;;
    Survived)
      echo "SURVIVED ${verdict#*$'\t'}"
      return 10
      ;;
    NoCoverage)
      echo "NOT COVERED ${verdict#*$'\t'}"
      return 1
      ;;
    *)
      echo "mutation.sh: no mutant is $id; the mutants of $declaration are in $report" >&2
      return 2
      ;;
  esac
}

case "${1:-}" in
  go)
    shift
    run_go "$@"
    ;;
  go-mutant)
    run_go_mutant "${2:-}"
    ;;
  frontend)
    shift
    run_frontend "$@"
    ;;
  frontend-mutant)
    run_frontend_mutant "${2:-}"
    ;;
  *)
    echo "usage: mutation.sh go <package directory>... | go-mutant <mutant> | frontend [<module>...] | frontend-mutant <mutant>" >&2
    exit 2
    ;;
esac
