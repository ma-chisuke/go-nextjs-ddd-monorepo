#!/usr/bin/env bash
# Breaking changes in the event schemas (TPL-0013). The Go tool in
# tools/cmd/events compares each schema in the working tree with the same file
# at the base commit (tools/scripts/merge-base.sh), and reports each schema of
# the base that is gone. The rules are in docs/guides/event-changes.md.
#
# Usage: files.sh "contracts/events/*.json" | events-breaking.sh
#
# A schema that does not exist at the base is new, and passes. Without a base,
# nothing is compared, and the check passes.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
root=$PWD

base=$(tools/scripts/merge-base.sh)
if [ -z "$base" ]; then
  echo "no base commit (CHECK_BASE=${CHECK_BASE:-origin/main}): nothing to compare"
  # Read the list anyway, so that the writer never meets a closed pipe.
  cat > /dev/null
  exit 0
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The event schemas of the base, offline, from Git, at their paths.
git ls-tree -r -z --name-only "$base" -- contracts/events/ | while IFS= read -r -d '' file; do
  case "$file" in
    *.json) ;;
    *) continue ;;
  esac
  mkdir -p "$tmp/$(dirname "$file")"
  git cat-file blob "$base:$file" > "$tmp/$file"
done

short=$(git rev-parse --short "$base")
cd tools
go run ./cmd/events -root "$root" -base "$tmp" -base-name "$short" breaking
