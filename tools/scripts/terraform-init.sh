#!/usr/bin/env bash
# `terraform init` in one directory, with the provider cache of the repository,
# one run at a time.
#
# Usage: terraform-init.sh <directory> [arguments of terraform init]...
#
# Run from the root of the repository. Every `terraform init` of the
# repository shares one provider cache, .cache/terraform/plugins, so that the
# AWS provider is downloaded once and not once per directory. Terraform does
# not guard that cache against two runs at once: the second one links a
# package that the first one is still writing, and fails on its checksum. In
# `make check-all`, validate-terraform and the smoke test both run
# `terraform init`, so this script takes a lock around it. The caller sets
# TF_DATA_DIR when the working data of Terraform must stay out of the
# directory.
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: terraform-init.sh <directory> [arguments of terraform init]..." >&2
  exit 2
fi
dir="$1"
shift

cache="$PWD/.cache/terraform"
export TF_PLUGIN_CACHE_DIR="$cache/plugins"
export TF_IN_AUTOMATION=1
export TF_INPUT=0
mkdir -p "$TF_PLUGIN_CACHE_DIR"

# A lock whose process is gone is taken over.
lock="$cache/init.lock"
tries=0
until mkdir "$lock" 2> /dev/null; do
  owner=$(cat "$lock/pid" 2> /dev/null || true)
  if [ -n "$owner" ] && ! kill -0 "$owner" 2> /dev/null; then
    rm -rf "$lock"
    continue
  fi
  tries=$((tries + 1))
  if [ "$tries" -gt 3000 ]; then
    echo "terraform-init: another terraform init has held $lock for 10 minutes; remove the directory if none runs" >&2
    exit 1
  fi
  sleep 0.2
done
echo "$$" > "$lock/pid"
# shellcheck disable=SC2064 # the path is fixed now
trap "rm -rf '$lock'" EXIT

terraform -chdir="$dir" init -no-color "$@"
