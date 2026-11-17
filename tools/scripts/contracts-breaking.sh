#!/usr/bin/env bash
# Breaking changes in the OpenAPI documents (TPL-0007, layer 6). oasdiff
# compares each document in the working tree with the same file at the base
# commit (tools/scripts/merge-base.sh), and each breaking change is a finding.
#
# Usage: files.sh "contracts/http/*.openapi.yaml" | contracts-breaking.sh
#
# A document that does not exist at the base is new, and passes. Without a
# base, nothing is compared, and the check passes.
#
# A deliberate breaking change raises info.version in the same change: below
# 1.0 the minor version (0.1.0 to 0.2.0), from 1.0 on the major version.
# oasdiff checks this itself (--fail-on MISSING-VERSION-BUMP). See
# docs/guides/api-changes.md.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

base=$(tools/scripts/merge-base.sh)
if [ -z "$base" ]; then
  echo "no base commit (CHECK_BASE=${CHECK_BASE:-origin/main}): nothing to compare"
  # Read the list anyway, so that the writer never meets a closed pipe.
  cat > /dev/null
  exit 0
fi
short=$(git rev-parse --short "$base")

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

status=0
while IFS= read -r doc; do
  if ! git cat-file -e "$base:$doc" 2> /dev/null; then
    echo "$doc: new since $short, nothing to compare"
    continue
  fi
  # oasdiff reads the base from Git itself, so references to other files
  # resolve in the same commit.
  if oasdiff breaking "$base:$doc" "$doc" --format json --fail-on MISSING-VERSION-BUMP \
    > "$tmp/out.json" 2> "$tmp/err"; then
    # Breaking changes with a raised version pass; oasdiff says so on stderr.
    grep -h 'policy: satisfied' "$tmp/err" | sed "s|^|$doc: |" || true
    continue
  fi
  status=1
  if jq -e 'type == "array"' "$tmp/out.json" > /dev/null 2>&1; then
    jq -r --arg doc "$doc" --arg base "$short" -f tools/oasdiff/findings.jq "$tmp/out.json"
  else
    # Not a report: the document could not be read.
    cat "$tmp/out.json" "$tmp/err"
  fi
done
exit "$status"
