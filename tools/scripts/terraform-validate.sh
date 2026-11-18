#!/usr/bin/env bash
# Validate each Terraform module given as an argument: `terraform init` without
# a backend, then `terraform validate`. One finding per line.
#
# Usage: terraform-validate.sh <module directory>...
#
# Run from the root of the repository. Provider plugins and the working data of
# Terraform live in .cache/terraform. A dependency lock file that
# `terraform init` creates in a module is removed again: a module that other
# configurations call does not keep one.
set -uo pipefail

root=$(pwd)
cache="$root/.cache/terraform"
export TF_PLUGIN_CACHE_DIR="$cache/plugins"
export TF_IN_AUTOMATION=1
export TF_INPUT=0
mkdir -p "$TF_PLUGIN_CACHE_DIR"

status=0
for dir in "$@"; do
  lock="$dir/.terraform.lock.hcl"
  had_lock=false
  if [ -f "$lock" ]; then
    had_lock=true
  fi
  export TF_DATA_DIR="$cache/data/$dir"
  # One init at a time: the smoke test runs one too (terraform-init.sh).
  if ! out=$("$root/tools/scripts/terraform-init.sh" "$dir" -backend=false 2>&1); then
    printf '%s\n' "$out"
    status=1
  elif ! terraform -chdir="$dir" validate -json -no-color \
    | jq -r --arg dir "$dir/" -f "$root/tools/terraform/findings.jq"; then
    status=1
  fi
  if [ "$had_lock" = false ]; then
    rm -f "$lock"
  fi
done
exit "$status"
