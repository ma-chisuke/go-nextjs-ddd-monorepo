#!/usr/bin/env bash
# The container images that an environment runs (TPL-0014): one for each
# bounded context, `web` for the frontend, and `db`, which prepares the
# database before a deployment.
#
# Usage: images.sh list                  the names of the images
#        images.sh tag <image>           the tag of the image
#        images.sh sources <image>       the paths that the image is built from
#        images.sh build <image> <ref>   build the image as <ref>, for linux/amd64
#        images.sh push <registry>       build and push each image whose tag the
#                                        registry does not hold yet; <registry>
#                                        is <account>.dkr.ecr.<region>.amazonaws.com
#
# Run from the root of the repository.
#
# The tag of an image is a hash of the files that it is built from, as Git
# holds them. A change to one context therefore gives a new tag to the image
# of that context alone, the task definitions of the other contexts stay as
# they are, and a deployment restarts only what changed. The paths of a
# context are complete because a context imports nothing of another context
# (`make lint-arch`): its command, its own packages, the platform packages,
# and the module files.
#
# `push` needs Docker, and AWS credentials that may push to Amazon ECR. The
# repositories are immutable, so an existing tag is never built again.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

contexts() {
  local dir
  for dir in backend/cmd/*/; do
    [ -f "${dir}main.go" ] || continue
    dir=${dir%/}
    printf '%s\n' "${dir##*/}"
  done
}

list() {
  contexts
  printf '%s\n' web db
}

# grep reads the whole list: with -q it would stop at the first match, and
# the list, still writing, would fail the pipeline (pipefail). That happened
# on a busy runner, for an image that is not the last of the list.
known() {
  list | grep -x -- "$1" > /dev/null || {
    echo "images: no image is named \"$1\"; the images are: $(list | tr '\n' ' ')" >&2
    exit 2
  }
}

sources() {
  local image="$1" file
  case "$image" in
    web)
      printf '%s\n' frontend package.json pnpm-lock.yaml pnpm-workspace.yaml
      ;;
    db)
      printf '%s\n' db
      for file in backend/internal/*/internal/adapter/postgres/schema.sql; do
        [ -f "$file" ] && printf '%s\n' "$file"
      done
      ;;
    *)
      printf '%s\n' "backend/cmd/$image" "backend/internal/$image" backend/internal/platform \
        backend/go.mod backend/go.sum backend/Dockerfile backend/.dockerignore
      ;;
  esac
}

# The names, modes, and contents of the files, as the index of Git has them.
tag() {
  local image="$1"
  # shellcheck disable=SC2046 # one argument for each path
  git ls-files -s -- $(sources "$image") | git hash-object --stdin | cut -c 1-16
}

build() {
  local image="$1" ref="$2"
  case "$image" in
    web)
      docker build --platform linux/amd64 --file frontend/Dockerfile --tag "$ref" .
      ;;
    db)
      docker build --platform linux/amd64 --file db/Dockerfile --tag "$ref" .
      ;;
    *)
      docker build --platform linux/amd64 --build-arg "CONTEXT=$image" --tag "$ref" backend
      ;;
  esac
}

push() {
  local registry="$1" image ref region dirty
  case "$registry" in
    *.dkr.ecr.*.amazonaws.com) ;;
    *)
      echo "images: \"$registry\" is not a registry of Amazon ECR, such as 111111111111.dkr.ecr.ap-northeast-1.amazonaws.com" >&2
      exit 2
      ;;
  esac
  region=${registry#*.dkr.ecr.}
  region=${region%.amazonaws.com}

  # A tag describes what Git holds; a file that differs from it would be
  # built into an image under the tag of another content.
  # shellcheck disable=SC2046 # one argument for each path
  dirty=$(git status --porcelain -- $(for image in $(list); do sources "$image"; done))
  if [ -n "$dirty" ]; then
    echo "images: commit or discard the changes to the sources of the images first:" >&2
    printf '%s\n' "$dirty" >&2
    exit 1
  fi

  aws ecr get-login-password --region "$region" \
    | docker login --username AWS --password-stdin "$registry" > /dev/null

  for image in $(list); do
    ref="$registry/$image:$(tag "$image")"
    if aws ecr describe-images --region "$region" --repository-name "$image" \
      --image-ids "imageTag=${ref##*:}" > /dev/null 2>&1; then
      echo "images: $ref is in the registry"
      continue
    fi
    echo "images: building $ref"
    build "$image" "$ref"
    docker push "$ref"
  done
}

case "${1:-}" in
  list)
    list
    ;;
  tag)
    known "${2:?usage: images.sh tag <image>}"
    tag "$2"
    ;;
  sources)
    known "${2:?usage: images.sh sources <image>}"
    sources "$2"
    ;;
  build)
    known "${2:?usage: images.sh build <image> <ref>}"
    build "$2" "${3:?usage: images.sh build <image> <ref>}"
    ;;
  push)
    push "${2:?usage: images.sh push <registry>}"
    ;;
  *)
    echo "usage: images.sh list|tag <image>|sources <image>|build <image> <ref>|push <registry>" >&2
    exit 2
    ;;
esac
