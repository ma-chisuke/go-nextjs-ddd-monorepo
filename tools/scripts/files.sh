#!/usr/bin/env bash
# Print the files a check should look at, one per line.
#
# Usage: files.sh <git pathspec>...
#   files.sh '*.sh'                 every shell script in scope
#   files.sh '*.md' ':!*.ja.md'     English Markdown only
#
# Scope comes from the environment, so every check narrows the same way:
#   CHECK_FILES   newline-separated paths; only these are considered
#   CHECK_SCOPE   all (default) | staged | changed
#                 staged:  files staged for the next commit
#                 changed: files that differ from CHECK_BASE (default
#                          origin/main), plus untracked files
#
# A change to a tool pin or to the checks themselves can change the result for
# any file, so when one of those files is staged or changed, every file is in
# scope.
#
# Paths matched by .lintignore are never printed. That file is the single
# place where vendored and generated files are excluded from every check that
# takes its files from this script.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

if [ "$#" -eq 0 ]; then
  set -- .
fi

pathspecs=("$@")
if [ -f .lintignore ]; then
  while IFS= read -r line; do
    case "$line" in
      '' | '#'*) continue ;;
    esac
    pathspecs+=(":(exclude)$line")
  done < .lintignore
fi

list_all() {
  git ls-files --cached --others --exclude-standard -- "${pathspecs[@]}"
}

# Tool pins, and the files that define the checks.
global_pathspecs=(
  mise.toml mise.lock
  package.json pnpm-lock.yaml pnpm-workspace.yaml
  pyproject.toml uv.lock
  tools/go.mod tools/go.sum
  Makefile make tools/scripts
  .lintignore .editorconfig
  ':(glob).*.json' ':(glob).*.jsonc' ':(glob).*.yaml' ':(glob).*.yml' ':(glob).*.toml' ':(glob).*.hcl' ':(glob).*.ini'
  ':(glob)*.toml' ':(glob)*.yaml'
  .vale
)

# Succeeds when the given `git diff` arguments report a change to one of them.
touches_global() {
  [ -n "$(git diff --name-only --diff-filter=ACMRD "$@" -- "${global_pathspecs[@]}" | head -n 1)" ]
}

list_scope() {
  case "${CHECK_SCOPE:-all}" in
    all)
      list_all
      ;;
    staged)
      if touches_global --cached; then
        list_all
      else
        git diff --cached --name-only --diff-filter=ACMR -- "${pathspecs[@]}"
      fi
      ;;
    changed)
      base="${CHECK_BASE:-origin/main}"
      if merge_base=$(git merge-base "$base" HEAD 2> /dev/null); then
        if touches_global "$merge_base"; then
          list_all
        else
          git diff --name-only --diff-filter=ACMR "$merge_base" -- "${pathspecs[@]}"
          git ls-files --others --exclude-standard -- "${pathspecs[@]}"
        fi
      else
        # No base to compare with (for example, a fresh clone): check everything.
        list_all
      fi
      ;;
    *)
      echo "files.sh: unknown CHECK_SCOPE '${CHECK_SCOPE}' (use all, staged, or changed)" >&2
      exit 2
      ;;
  esac
}

if [ -n "${CHECK_FILES:-}" ]; then
  # Keep only the requested files that also match the pathspecs.
  list_all | grep -Fx -f <(printf '%s\n' "$CHECK_FILES") || true
else
  list_scope
fi | sort -u | while IFS= read -r path; do
  # A tracked file can be deleted in the working tree; skip it.
  if [ -f "$path" ]; then
    # A reader that has seen enough (`| head -n 1`) closes the pipe; stop quietly.
    printf '%s\n' "$path" 2> /dev/null || exit 0
  fi
done
