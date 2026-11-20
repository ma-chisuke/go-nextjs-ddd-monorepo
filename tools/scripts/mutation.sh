#!/usr/bin/env bash
# Mutation testing (docs/guides/mutation-tests.md). gomutants changes the code
# of Go packages in small ways, one change (a mutant) at a time, and runs the
# tests of each package against each mutant. A mutant that every test passes
# shows a rule that the tests do not hold, and fails the check.
#
# Usage:
#   mutation.sh go <package directory>...   every mutant of the packages
#   mutation.sh go-mutant <mutant>          one mutant, named as a finding names it
#
# Each mutant runs only the tests of its own package, so the tests of a package
# must hold its rules themselves. equivalent-mutants.allow names the mutants
# that a person showed to be equivalent: changes that no test can tell apart
# from the code. See tools/gomutants/findings.jq.
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

  # An allowance that names a file that is gone, outside the packages of the
  # run; findings.jq reports those inside them.
  if [ -f "$allow_file" ]; then
    local n=0
    while IFS= read -r line; do
      n=$((n + 1))
      case "$line" in
        '' | '#'*) continue ;;
      esac
      id=${line%% | *}
      file=${id%%:*}
      case " $* " in
        *" $(dirname "$file") "*) continue ;;
      esac
      if [ ! -f "$file" ]; then
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

case "${1:-}" in
  go)
    shift
    run_go "$@"
    ;;
  go-mutant)
    run_go_mutant "${2:-}"
    ;;
  *)
    echo "usage: mutation.sh go <package directory>... | go-mutant <mutant>" >&2
    exit 2
    ;;
esac
