#!/usr/bin/env bash
# AWS Emulator in Docker Compose, for the tests and the tools that run on this
# host (TPL-0017). The services of compose.yaml reach it by themselves.
#
# Usage: aws.sh server   start the emulator unless its container exists, wait
#                        until it is healthy, and copy the CA of its endpoint
#                        to .cache/aws-emulator/ca.pem
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
  mkdir -p "$(dirname "$ca")"
  docker compose cp aws:/home/emulator/.aws/emulator/9297/ca.pem "$ca.$$" > /dev/null 2>&1 || {
    echo "aws: cannot copy the CA of AWS Emulator from its container" >&2
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

case "${1:-}" in
  server)
    server
    ;;
  env)
    environment
    ;;
  *)
    echo "usage: aws.sh server|env" >&2
    exit 2
    ;;
esac
