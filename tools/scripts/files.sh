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
# Paths matched by .lintignore are never printed. That file is the single
# place where vendored and generated files are excluded from every check.
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

list_scope() {
  case "${CHECK_SCOPE:-all}" in
    all)
      list_all
      ;;
    staged)
      git diff --cached --name-only --diff-filter=ACMR -- "${pathspecs[@]}"
      ;;
    changed)
      base="${CHECK_BASE:-origin/main}"
      if merge_base=$(git merge-base "$base" HEAD 2> /dev/null); then
        git diff --name-only --diff-filter=ACMR "$merge_base" -- "${pathspecs[@]}"
        git ls-files --others --exclude-standard -- "${pathspecs[@]}"
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
    printf '%s\n' "$path"
  fi
done
