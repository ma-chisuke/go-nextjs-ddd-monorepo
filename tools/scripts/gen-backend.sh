#!/usr/bin/env bash
# Generate the Go code of the backend from its sources of truth, or check
# that the generated code is up to date.
#
# Usage: gen-backend.sh api  [--check]    oapi-codegen, from contracts/http/
#        gen-backend.sh sqlc [--check]    sqlc, from schema.sql and queries/
#
# api: every oapi-codegen.yaml generates the file that it names, next to
# itself. Which API document it reads follows from where it is:
#   backend/internal/<context>/internal/adapter/http/      the API of <context>
#   backend/internal/<context>/internal/adapter/<other>/   the API of <other>,
#                                                           as a client
# sqlc: every sqlc.yaml generates next to itself.
#
# With --check nothing is written. A generated file that differs from what
# the generator writes now is a finding, and the exit status is 1.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
root=$PWD
kind="${1:?usage: gen-backend.sh api|sqlc [--check]}"
check="${2:-}"
status=0

configs() {
  git ls-files --cached --others --exclude-standard -- "backend/**/$1" "backend/$1"
}

stale() {
  echo "$1:1: [gen/stale] the generated file is not what the generator writes from $2. Fix: run make gen. See TPL-0007."
  status=1
}

gen_api() {
  local config dir name context document output tmp
  while IFS= read -r config; do
    [ -f "$config" ] || continue
    dir=$(dirname "$config")
    name=$(basename "$dir")
    if [ "$name" = http ]; then
      context=${config#backend/internal/}
      context=${context%%/*}
    else
      context=$name
    fi
    document="contracts/http/$context.openapi.yaml"
    if [ ! -f "$document" ]; then
      echo "$config:1: [gen/no-document] $document does not exist. Fix: add the API document of the context '$context', or move this file to adapter/http or adapter/<context>."
      status=1
      continue
    fi
    output=$(sed -n 's/^output: *//p' "$config")
    if [ "$check" = --check ]; then
      # The same configuration, writing to a temporary file.
      tmp=$(mktemp)
      sed "s|^output: .*|output: $tmp.go|" "$config" > "$tmp.yaml"
      (cd "$dir" && go tool -modfile="$root/tools/go.mod" oapi-codegen -config "$tmp.yaml" "$root/$document")
      cmp -s "$tmp.go" "$dir/$output" || stale "$dir/$output" "$document"
      rm -f "$tmp" "$tmp.yaml" "$tmp.go"
    else
      (cd "$dir" && go tool -modfile="$root/tools/go.mod" oapi-codegen -config oapi-codegen.yaml "$root/$document")
    fi
  done < <(configs oapi-codegen.yaml)
}

gen_sqlc() {
  local config dir
  while IFS= read -r config; do
    [ -f "$config" ] || continue
    dir=$(dirname "$config")
    if [ "$check" = --check ]; then
      (cd "$dir" && sqlc diff > /dev/null 2>&1) || stale "$dir/*.gen.go" "$dir/schema.sql and $dir/queries"
    else
      (cd "$dir" && sqlc generate)
    fi
  done < <(configs sqlc.yaml)
}

case "$kind" in
  api) gen_api ;;
  sqlc) gen_sqlc ;;
  *)
    echo "gen-backend.sh: unknown kind '$kind' (use api or sqlc)" >&2
    exit 2
    ;;
esac
exit "$status"
