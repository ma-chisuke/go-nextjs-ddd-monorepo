#!/usr/bin/env bash
# Report each page that a change adds without a scenario in a browser
# (TPL-0023).
#
# What a new page does is tested in a browser: its components and its Server
# Functions have no other test. A guide alone did not hold that rule, so this
# check holds the part of it that a script can decide. A change that adds a
# `page.tsx` under frontend/src/app/ also adds or changes a scenario,
# frontend/e2e/*.spec.ts. Whether the scenario opens the page is for review:
# the check only knows that the change came with one.
#
# The change is what differs from the commit that tools/scripts/merge-base.sh
# prints, files that Git does not track yet included. Without that commit,
# as in a clone without the base branch, there is nothing to compare.
#
# Usage: new-pages.sh   (from the root of the repository)
set -euo pipefail

base=$(tools/scripts/merge-base.sh)
if [ -z "$base" ]; then
  exit 0
fi

# changed <filter> <directory>: the files of the directory that the change
# adds (A) or adds and modifies (AM).
changed() {
  git diff --name-only --diff-filter="$1" "$base" -- "$2"
  git ls-files --others --exclude-standard -- "$2"
}

pages=$(changed A frontend/src/app | grep -E '(^|/)page\.tsx$' | sort -u || true)
if [ -z "$pages" ]; then
  exit 0
fi
scenarios=$(changed AM frontend/e2e | grep -E '\.spec\.ts$' || true)
if [ -n "$scenarios" ]; then
  exit 0
fi

while IFS= read -r page; do
  echo "$page:1: [e2e/new-page] the change adds this page, and adds or changes no scenario in frontend/e2e/. Fix: write a scenario that opens the page and does what a person does there (docs/guides/add-a-screen.md, step 7). See TPL-0023."
done <<< "$pages"
exit 1
