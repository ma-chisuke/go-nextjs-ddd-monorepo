#!/usr/bin/env bash
# Report every ignore comment that review did not allow (TPL-0020).
#
# An ignore comment turns a check off for one place: `//nolint` for
# golangci-lint, a `disable` directive for ShellCheck, `eslint-disable`, and
# the like. A change that adds one passes the check without meeting it, so
# the comments are counted here. ignore-comments.allow names each comment
# that review let in, with the reason why the check is wrong for that code:
#
#   <file> | <ignore comment> | <reason>
#
# The script fails on a comment that the file does not name, on a line of the
# file that names no comment any more, and on a line without a reason. It
# reads every file of the repository, tracked or not yet, whatever the scope
# of the run: a line of the file is used or not by the whole repository.
# Generated code is left out; its generator writes its comments.
#
# Usage: ignore-comments.sh   (from the root of the repository)
set -euo pipefail

allow_file=ignore-comments.allow

# found <pattern> <glob>...: "<file>:<line>:<match>" for each match of the
# pattern in the files that the globs name.
found() {
  local pattern="$1" status=0
  shift
  git grep --untracked -I -n -o -E -e "$pattern" -- "$@" || status=$?
  # git grep exits with 1 when nothing matches.
  [ "$status" -le 1 ]
}

# The comments of each tool that the repository pins, in the files that the
# tool reads. A pattern matches the comment itself, not a text that names it:
# a directive for ShellCheck is a comment line, and one for Markdown an HTML
# comment.
comments() {
  found '//[[:space:]]*nolint(:[A-Za-z0-9_,-]+)?|#nosec' \
    '*.go'
  found '^[[:space:]]*#[[:space:]]*shellcheck[[:space:]]+disable=[A-Za-z0-9,]+' \
    '*.sh'
  found 'eslint-disable(-next-line|-line)?|@ts-ignore|@ts-expect-error|@ts-nocheck' \
    '*.ts' '*.tsx' '*.js' '*.jsx' '*.mjs' '*.cjs'
  found 'prettier-ignore[a-z-]*' \
    '*.ts' '*.tsx' '*.js' '*.jsx' '*.mjs' '*.cjs' '*.css' '*.json' '*.jsonc' '*.json5' '*.yaml' '*.yml'
  found 'trivy:ignore:[A-Za-z0-9_-]+|tflint-ignore:[[:space:]]*[A-Za-z0-9_,-]+' \
    '*.tf' '*.tfvars' '*.hcl' '*.yaml' '*.yml' '*Dockerfile' '*Dockerfile.*' '*.Dockerfile'
  found 'hadolint[[:space:]]+ignore=[A-Za-z0-9,]+' \
    '*Dockerfile' '*Dockerfile.*' '*.Dockerfile'
  found 'yamllint[[:space:]]+disable[a-z-]*|zizmor:[[:space:]]*ignore\[[^]]*\]' \
    '*.yaml' '*.yml'
  found '<!--[[:space:]]*(markdownlint-disable[a-z-]*|textlint-disable|vale[[:space:]]+off|prettier-ignore[a-z-]*)' \
    '*.md'
  found 'squawk-ignore[a-z-]*' \
    '*.sql'
}

# The comment as a line of the file names it: without its comment sign, and
# with one space where it has any.
comments \
  | grep -v -E '^[^:]*\.gen\.(go|ts):' \
  | sed -E \
    -e 's/^([^:]*:[0-9]+:)[[:space:]]*(\/\/|#[[:space:]]|<!--)[[:space:]]*/\1/' \
    -e 's/[[:space:]]+/ /g' \
  | sort -t : -k 1,1 -k 2,2n \
  | awk -v allow="$allow_file" '
    function trim(s) {
      gsub(/^[ \t]+|[ \t]+$/, "", s)
      return s
    }
    BEGIN {
      n = 0
      while ((getline line < allow) > 0) {
        n++
        if (line ~ /^[ \t]*(#|$)/) {
          continue
        }
        count = split(line, part, "|")
        file = trim(part[1])
        comment = trim(part[2])
        reason = ""
        for (i = 3; i <= count; i++) {
          reason = reason (i > 3 ? "|" : "") part[i]
        }
        if (file == "" || comment == "" || trim(reason) == "") {
          printf "%s:%d: [ignore-comments/allowance-format] the line is not \"<file> | <ignore comment> | <reason>\". Fix: write the file, the comment as a finding of this check names it, and the reason why the check is wrong for that code. See TPL-0020.\n", allow, n
          failed = 1
          continue
        }
        key = file SUBSEP comment
        allowed[key]++
        at[key, allowed[key]] = n
      }
    }
    {
      i = index($0, ":")
      file = substr($0, 1, i - 1)
      rest = substr($0, i + 1)
      j = index(rest, ":")
      key = file SUBSEP substr(rest, j + 1)
      seen[key]++
      if (seen[key] > allowed[key]) {
        printf "%s:%s: [ignore-comments/not-allowed] \"%s\" turns a check off for this code. Fix: change the code so that the check passes, and remove the comment. If the check is wrong for this code, stop and say so: an ignore comment is a decision of review, which %s records. See TPL-0020.\n", file, substr(rest, 1, j - 1), substr(rest, j + 1), allow
        failed = 1
      }
    }
    END {
      for (key in allowed) {
        split(key, name, SUBSEP)
        for (c = seen[key] + 1; c <= allowed[key]; c++) {
          printf "%s:%d: [ignore-comments/unused-allowance] %s holds no \"%s\" for this line. Fix: remove the line. See TPL-0020.\n", allow, at[key, c], name[1], name[2]
          failed = 1
        }
      }
      exit failed
    }'
