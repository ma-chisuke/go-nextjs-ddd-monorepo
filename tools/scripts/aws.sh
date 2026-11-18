#!/usr/bin/env bash
# AWS Emulator in Docker Compose (TPL-0017): the emulator itself, for the
# tests and the tools that run on this host, and the AWS resources of the
# local system in it. The services of compose.yaml reach the emulator by
# themselves.
#
# Usage: aws.sh server   start the emulator unless its container exists, wait
#                        until it is healthy, and copy the CA of its endpoint
#                        to .cache/aws-emulator/ca.pem
#        aws.sh apply    apply infra/local/ to the running emulator with
#                        Terraform: the event bus, and the queue and the rule
#                        of each context that consumes events
#        aws.sh env      print the environment that points the AWS SDKs and
#                        Terraform at the emulator, as `export` lines for eval
#
# The endpoint is HTTPS on the loopback address, port AWS_PORT (59297). The
# emulator makes its certificate and the CA at every start, so `server` copies
# the CA every time. Requests are not signed, and the credentials of `env` are
# placeholders that the SDKs need to sign at all.
#
# `server` leaves an existing container alone, also one that another checkout
# (a Git worktree) started: the emulator keeps its state in memory, and
# recreating it would take the queues away from whoever uses them. So the
# tests name what they create uniquely. `make up` recreates the container when
# its image or compose.yaml changed.
#
# `apply` uses the modules that the environments in AWS use, so the local
# queues and rules cannot drift from the real ones. The emulator forgets
# everything when it stops, so the Terraform state lives only as long as the
# container: it is kept outside the repository, in a directory that every
# checkout on this machine shares, as they share the emulator, and it is
# thrown away when the container is not the one that the state describes.
# The first apply after a start takes about a minute, because the AWS provider
# waits half a minute after it creates a queue; an apply without changes
# takes a few seconds.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

port="${AWS_PORT:-59297}"
ca=".cache/aws-emulator/ca.pem"

server() {
  local log
  log=$(mktemp)
  if ! docker compose up --build --detach --wait --wait-timeout 120 --no-recreate --quiet-pull aws > "$log" 2>&1; then
    cat "$log" >&2
    rm -f "$log"
    echo "aws: AWS Emulator did not become healthy; its last log lines:" >&2
    docker compose logs --no-color --tail 20 aws >&2
    exit 1
  fi
  rm -f "$log"
  copy_ca
}

copy_ca() {
  mkdir -p "$(dirname "$ca")"
  docker compose cp aws:/home/emulator/.aws/emulator/9297/ca.pem "$ca.$$" > /dev/null 2>&1 || {
    echo "aws: cannot copy the CA of AWS Emulator from its container; start it with \`make aws-server\` or \`make up\`" >&2
    exit 1
  }
  mv "$ca.$$" "$ca"
}

environment() {
  cat << EOF
export AWS_ENDPOINT_URL=https://localhost:$port
export AWS_CA_BUNDLE=$PWD/$ca
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=emulator
export AWS_SECRET_ACCESS_KEY=emulator
EOF
}

apply() {
  local container started shared state marker log
  container=$(docker compose ps --quiet aws)
  if [ -z "$container" ]; then
    echo "aws: AWS Emulator does not run; start it with \`make aws-server\` or \`make up\`" >&2
    exit 1
  fi
  copy_ca
  started=$(docker inspect --format '{{.Id}} {{.State.StartedAt}}' "$container")

  shared="${TMPDIR:-/tmp}"
  shared="${shared%/}/aws.sh-$port"
  state="$shared/terraform.tfstate"
  marker="$shared/emulator"
  mkdir -p "$shared"
  if [ "$(cat "$marker" 2> /dev/null || true)" != "$started" ]; then
    # Another container, or the same one after a restart: it holds nothing
    # of what the state describes.
    rm -f "$state" "$state.backup"
    printf '%s\n' "$started" > "$marker"
  fi

  eval "$(environment)"
  # The working data of this apply, apart from that of validate-terraform,
  # which initializes the same directory without a backend at the same time
  # in `make check-all`.
  export TF_DATA_DIR="$PWD/.cache/terraform/apply/infra/local"
  export TF_PLUGIN_CACHE_DIR="$PWD/.cache/terraform/plugins"
  export TF_IN_AUTOMATION=1
  export TF_INPUT=0

  log=$(mktemp)
  # The providers are the ones that infra/local/.terraform.lock.hcl names. A
  # second apply at the same time, from `make check-all` in another checkout,
  # waits for the lock of the state.
  if ! {
    tools/scripts/terraform-init.sh infra/local -lockfile=readonly -reconfigure \
      -backend-config="path=$state" \
      && terraform -chdir=infra/local apply -no-color -auto-approve -lock-timeout=300s \
        -var "endpoint=$AWS_ENDPOINT_URL" -var "ca_bundle=$AWS_CA_BUNDLE"
  } > "$log" 2>&1; then
    cat "$log" >&2
    rm -f "$log"
    echo "aws: Terraform could not apply infra/local to AWS Emulator" >&2
    exit 1
  fi
  rm -f "$log"
}

case "${1:-}" in
  server)
    server
    ;;
  apply)
    apply
    ;;
  env)
    environment
    ;;
  *)
    echo "usage: aws.sh server|apply|env" >&2
    exit 2
    ;;
esac
