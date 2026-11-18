#!/usr/bin/env bash
# Generate the TypeScript types of the frontend from the API documents, or
# check that the generated types are up to date (TPL-0006, TPL-0007).
#
# Usage: gen-frontend.sh api [--check]    openapi-typescript, from contracts/http/
#
# api: every contracts/http/<context>.openapi.yaml generates
# frontend/src/lib/api/<context>.gen.ts, the paths and the schemas of the API
# of that context. A generated file whose document is gone is removed.
#
# With --check nothing is written. A generated file that differs from what
# the generator writes now, that is missing, or that has no document is a
# finding, and the exit status is 1.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

kind="${1:?usage: gen-frontend.sh api [--check]}"
check="${2:-}"
out=frontend/src/lib/api
generator=frontend/node_modules/.bin/openapi-typescript
status=0

finding() {
  echo "$1:1: [$2] $3 Fix: run make gen. See TPL-0007."
  status=1
}

gen_api() {
  local document context target tmp
  for document in contracts/http/*.openapi.yaml; do
    [ -f "$document" ] || continue
    context=$(basename "$document" .openapi.yaml)
    target="$out/$context.gen.ts"
    if [ "$check" = --check ]; then
      tmp=$(mktemp)
      "$generator" "$document" --output "$tmp" > /dev/null
      if [ ! -f "$target" ]; then
        finding "$document" gen/missing "the types of this API document, $target, do not exist."
      elif ! cmp -s "$tmp" "$target"; then
        finding "$target" gen/stale "the generated file is not what the generator writes from $document."
      fi
      rm -f "$tmp"
    else
      mkdir -p "$out"
      "$generator" "$document" --output "$target" > /dev/null
    fi
  done
  # The types of a context whose API document is gone.
  for target in "$out"/*.gen.ts; do
    [ -f "$target" ] || continue
    context=$(basename "$target" .gen.ts)
    if [ ! -f "contracts/http/$context.openapi.yaml" ]; then
      if [ "$check" = --check ]; then
        finding "$target" gen/no-document "contracts/http/$context.openapi.yaml does not exist, so nothing generates this file."
      else
        rm -f "$target"
      fi
    fi
  done
}

case "$kind" in
  api) gen_api ;;
  *)
    echo "gen-frontend.sh: unknown kind '$kind' (use api)" >&2
    exit 2
    ;;
esac
exit "$status"
