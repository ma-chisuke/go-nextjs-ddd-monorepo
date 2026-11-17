#!/usr/bin/env bash
# Report every base image in the given Dockerfiles that is not pinned by
# digest (TPL-0024): each FROM names `<image>:<tag>@sha256:<digest>`, so a
# moved tag changes no image, and Renovate proposes every new digest. A FROM
# that names an earlier stage, scratch, or a build argument is not an image
# to pin. One finding per line.
#
# Usage: image-pins.sh <Dockerfile>...
set -euo pipefail

awk '
  FNR == 1 { split("", stages) }
  toupper($1) == "FROM" {
    i = 2
    while (i <= NF && $i ~ /^--/) {
      i++
    }
    image = $i
    stage = (i + 2 <= NF && toupper($(i + 1)) == "AS") ? tolower($(i + 2)) : ""
    if (image != "scratch" && image !~ /^\$/ && !(tolower(image) in stages) && image !~ /@sha256:[0-9a-f]+$/) {
      printf "%s:%d: [image-pins] the base image %s is not pinned by digest. Fix: append @sha256:<digest> of its index, which `docker buildx imagetools inspect %s` prints (TPL-0024)\n", FILENAME, FNR, image, image
      failed = 1
    }
    if (stage != "") {
      stages[stage] = 1
    }
  }
  END { exit failed }
' "$@"
