#!/usr/bin/env bash
# The self-test of the checks. Every check target must fail on a known-bad
# input and name the expected rule, and pass on a known-good input. The cases
# run the real make targets, so they test the wiring (which files a target
# selects, its scope, its output), not only the tools.
#
# Usage: make selftest                          every case
#        make selftest SELFTEST_ONLY=<text>     the cases whose name holds <text>
#        make selftest VERBOSE=1                also what each check printed
#
# How it works: the tracked files and the untracked files that Git does not
# ignore are copied to a temporary directory outside the working tree, with
# node_modules and .venv linked, and committed to a new Git repository. Each
# case gets its own copy of it, writes its fixtures there, and runs
# `make <target>` in it. Cases run in parallel, $JOBS at a time.
#
# Add a case: write a function named case_<target>, with the dashes of the
# target written as underscores. Another case for the same target adds
# __<variant> to the name. The name decides the target under test:
#
#   case_lint_sql() {
#     put db/schema.sql 'CREATE TABLE t (name varchar(10));'
#     fails 'prefer-text-field'      # `make lint-sql` fails and names the rule
#     put db/schema.sql 'CREATE TABLE t (name text);'
#     passes                         # `make lint-sql` passes
#   }
#
# The helpers:
#   put <path> [<text>]           write a file in the copy: the text, or stdin
#   drop <path>                   remove a file or a directory from the copy
#   run <command>...              run a command in the copy (`run git add a.sh`)
#   commit <message>              commit what is staged in the copy
#   fails <text> [VAR=value]...   run the target: it must fail and print <text>
#   passes [VAR=value]...         run the target: it must pass
#   prints <text>, omits <text>   the output of the last run holds, or lacks, <text>
#
# Every target that make passes as an argument (CHECK_TARGETS) needs a case, or
# the self-test fails. A fixture that would trip the checks of this repository
# itself, such as a misspelling or a secret, is put together at run time, so
# that this file stays clean.
set -uo pipefail

case "$0" in
  /*) self=$0 ;;
  *) self="$PWD/$0" ;;
esac
repo=$(git rev-parse --show-toplevel) || exit 2
cd "$repo" || exit 2

jobs="${JOBS:-8}"
only="${SELFTEST_ONLY:-}"
verbose="${VERBOSE:-0}"
make_cmd="${MAKE:-make}"

# Nothing from the calling make or the scope of a check may leak into the copies.
unset MAKEFLAGS MFLAGS MAKELEVEL MAKEOVERRIDES CHECK_SCOPE CHECK_FILES CHECK_BASE CHECK_STATE_DIR VERBOSE
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX

# ---------------------------------------------------------------------------
# Helpers for the cases. A case runs in a subshell, in its copy ($COPY), with
# the target under test in $TARGET.

die() {
  printf 'selftest: %s\n' "$*"
  exit 1
}

put() {
  mkdir -p "$(dirname "$COPY/$1")" || die "cannot create the directory of $1"
  if [ "$#" -ge 2 ]; then
    printf '%s\n' "$2" > "$COPY/$1"
  else
    cat > "$COPY/$1"
  fi
}

drop() {
  rm -rf "${COPY:?}/$1"
}

run() {
  (cd "$COPY" && "$@") > "$OUT" 2>&1 || {
    cat "$OUT"
    die "command failed: $*"
  }
}

commit() {
  run git commit --quiet --no-verify --message "$1"
}

# Run the target under test with the given variables.
make_target() {
  (cd "$COPY" && env "$@" "$make_cmd" --no-print-directory "$TARGET") > "$OUT" 2>&1
  STATUS=$?
  LAST="make $TARGET${*:+ with $*}"
}

report() {
  printf '%s\n--- output of %s:\n' "$1" "$LAST"
  cat "$OUT"
  exit 1
}

fails() {
  local text=$1
  shift
  make_target "$@"
  if [ "$STATUS" -eq 0 ]; then
    report "expected $LAST to fail, but it passed"
  fi
  prints "$text"
  if [ "$verbose" = 1 ]; then
    printf -- '--- %s:\n' "$LAST"
    cat "$OUT"
  fi
}

passes() {
  make_target "$@"
  if [ "$STATUS" -ne 0 ]; then
    report "expected $LAST to pass, but it failed"
  fi
}

prints() {
  grep -qF -- "$1" "$OUT" || report "expected the output of $LAST to name '$1'"
}

omits() {
  if grep -qF -- "$1" "$OUT"; then
    report "expected the output of $LAST not to name '$1'"
  fi
}

# ---------------------------------------------------------------------------
# Cases: the files that every check looks at, and the scope of tools/scripts/files.sh.

case_fmt_check_prettier() {
  put selftest.json '{"a":1}'
  fails '[warn] selftest.json'
  put selftest.json '{ "a": 1 }'
  passes
}

case_fmt_check_toml() {
  put selftest.toml 'a=1'
  fails 'selftest.toml'
  put selftest.toml 'a = 1'
  passes
}

case_lint_toml() {
  put selftest.toml 'a = '
  fails 'selftest.toml'
  put selftest.toml 'a = 1'
  passes
}

case_fmt_check_shell() {
  put selftest.sh << 'EOF'
#!/usr/bin/env bash
if true; then
echo ok
fi
EOF
  fails 'selftest.sh'
  put selftest.sh << 'EOF'
#!/usr/bin/env bash
if true; then
  echo ok
fi
EOF
  passes
}

case_lint_shell() {
  put selftest.sh << 'EOF'
#!/usr/bin/env bash
echo $1
EOF
  fails 'SC2086'
  put selftest.sh << 'EOF'
#!/usr/bin/env bash
echo "$1"
EOF
  passes
}

case_lint_yaml() {
  put selftest.yaml << 'EOF'
a: 1
a: 2
EOF
  fails 'key-duplicates'
  put selftest.yaml 'a: 1'
  passes
}

case_lint_schema() {
  put compose.yaml << 'EOF'
services:
  web:
    image: nginx:1.29
    restart_policy: always
EOF
  fails 'compose.yaml'
  put compose.yaml << 'EOF'
services:
  web:
    image: nginx:1.29
    restart: always
EOF
  passes
}

case_lint_dockerfile() {
  put Dockerfile << 'EOF'
FROM ubuntu:latest
USER 65534
EOF
  fails 'DL3007'
  put Dockerfile << 'EOF'
FROM alpine:3.22
USER 65534
EOF
  passes
}

# The Dockerfile of the backend is in scope, and passes as it is.
case_lint_dockerfile__backend() {
  local dockerfile
  dockerfile=$(sed 's/^FROM gcr.io[^ ]*/FROM ubuntu:latest/' "$COPY/backend/Dockerfile")
  put backend/Dockerfile "$dockerfile"
  fails 'backend/Dockerfile'
  prints 'DL3007'
  run git checkout -- backend/Dockerfile
  passes
}

case_lint_typos() {
  local word
  word=$(printf 't%s' 'eh')
  put notes.txt "A sentence with $word word."
  fails "\`$word\`"
  put notes.txt 'A sentence with the word.'
  passes
}

case_lint_editorconfig() {
  put notes.txt "$(printf 'Trailing space %s' '')"
  fails 'notes.txt'
  put notes.txt 'No trailing space'
  passes
}

# An AWS access key ID that looks real, put together at run time.
fake_secret() {
  printf 'aws_access_key_id = %s%s\n' 'AKIA' 'Q2X7K3ZL4MWP5RT6'
}

case_lint_secrets() {
  put config/credentials "$(fake_secret)"
  fails 'aws-access-token'
  omits 'Q2X7K3ZL4MWP5RT6'
  put config/credentials 'The access key comes from the environment.'
  passes
}

case_lint_secrets__staged() {
  put config/staged "$(fake_secret)"
  run git add config/staged
  fails 'aws-access-token' CHECK_SCOPE=staged
  # The staged mode looks at the staged changes only.
  run git reset --quiet config/staged
  passes CHECK_SCOPE=staged
}

case_lint_md() {
  put notes.md << 'EOF'
# Notes

### Skipped a level
EOF
  fails 'MD001'
  put notes.md << 'EOF'
# Notes

## One level down
EOF
  passes
}

case_lint_prose_en() {
  put notes.md << 'EOF'
# Notes

Format the files, e.g. with Prettier.
EOF
  fails 'Google.Latin'
  put notes.md << 'EOF'
# Notes

Format the files, for example with Prettier.
EOF
  passes
}

case_lint_prose_ja() {
  put notes.ja.md << 'EOF'
# メモ

この文書は日本語で書かれています。
EOF
  fails 'no-mix-dearu-desumasu'
  put notes.ja.md << 'EOF'
# メモ

この文書は日本語で書く。
EOF
  passes
}

case_lint_adr() {
  put docs/adr/ADR-0001-try-something.md << 'EOF'
---
status: done
date: 2026-11-17
---

# ADR-0001: Try something
EOF
  fails 'adr/status'
  drop docs/adr/ADR-0001-try-something.md
  passes
}

case_lint_adr__template() {
  local record=docs/adr/template/TPL-0001-record-decisions-as-adrs.md
  printf '\nA team edited this template record.\n' >> "$COPY/$record"
  fails 'adr/template-changed'
  run git checkout -q -- "$record"
  passes
}

case_lint_i18n() {
  put docs/guides/extra.md << 'EOF'
# Extra

A guide that has no translation yet.
EOF
  fails 'i18n/missing-translation'
  # A translation whose marker holds the hash of an older English file.
  local stale
  stale=$(printf '%064d' 0)
  put docs/guides/extra.ja.md << EOF
# 追加

まだ対訳のなかったガイドである。

<!-- i18n-source: extra.md sha256:$stale -->
EOF
  fails 'i18n/stale-translation'
  run "$make_cmd" --no-print-directory i18n-stamp
  passes
}

case_lint_agents_md() {
  local n=0
  {
    printf '# AGENTS.md\n\n'
    while [ "$n" -lt 80 ]; do
      n=$((n + 1))
      printf -- '- Rule %s.\n' "$n"
    done
  } > "$COPY/AGENTS.md"
  fails 'agents-md/too-long'
  run git checkout -q -- AGENTS.md
  passes
}

case_test_hooks() {
  # A guard that allows everything must fail the tests of the hooks.
  put .claude/hooks/guard-read.sh << 'EOF'
#!/usr/bin/env bash
exit 0
EOF
  fails 'guard-read.sh'
  run git checkout -q -- .claude/hooks/guard-read.sh
  passes
}

case_gen_check_api() {
  # A generated file that somebody edited by hand, or forgot to regenerate.
  local generated=backend/internal/facility/internal/adapter/http/api.gen.go
  printf '\n// Edited by hand.\n' >> "$COPY/$generated"
  fails 'gen/stale'
  run git checkout -q -- "$generated"
  passes
}

case_gen_check_sqlc() {
  # A query that changed without `make gen`.
  local queries=backend/internal/facility/internal/adapter/postgres/queries/rooms.sql
  put "$queries" << 'EOF'
-- name: GetRoomByName :one
SELECT id, name, capacity, open_from, open_until, time_zone, created_at
FROM rooms
WHERE name = sqlc.arg(name);
EOF
  fails 'gen/stale'
  run git checkout -q -- "$queries"
  passes
}

# The architecture of the backend: one case for each rule of tools/cmd/arch.
# The rules read Go files without compiling them, so these fixtures need not
# build, and their indentation does not matter.

# The module path of the backend, which the imports of the fixtures start with.
backend_module() {
  sed -n 's/^module //p' "$COPY/backend/go.mod"
}

# A core context next to Facility: a domain layer, and its glossary.
put_core_context() {
  put backend/internal/selftest/internal/domain/visit.go << 'EOF'
// Package domain is a fixture of the self-test.
package domain

// Visit is a fixture.
type Visit struct{}
EOF
  put docs/contexts/selftest/glossary.md << 'EOF'
# Selftest glossary

| Identifier | Term | Meaning |
|---|---|---|
| `Visit` | visit | A fixture. |
EOF
  put docs/contexts/selftest/glossary.ja.md << 'EOF'
# selftest の用語集

| 識別子 | 用語 | 意味 |
|---|---|---|
| `Visit` | 訪問 | 自己テストの部品。 |
EOF
}

case_lint_arch() {
  # A test of Facility that imports the domain of another context.
  put_core_context
  local module
  module=$(backend_module)
  put backend/internal/facility/internal/app/visit_test.go << EOF
package app

import _ "$module/internal/selftest/internal/domain"
EOF
  fails 'arch/context-import'
  drop backend/internal/facility/internal/app/visit_test.go
  passes
}

case_lint_arch__module_import() {
  # A command that wires Facility, but is not cmd/facility.
  local module
  module=$(backend_module)
  put backend/cmd/report/main.go << EOF
package main

import _ "$module/internal/facility"

func main() {}
EOF
  fails 'arch/module-import'
  drop backend/cmd/report
  passes
}

case_lint_arch__platform_import() {
  local module
  module=$(backend_module)
  put backend/internal/platform/rooms/rooms.go << EOF
package rooms

import _ "$module/internal/facility/internal/app"
EOF
  fails 'arch/platform-import'
  drop backend/internal/platform/rooms
  passes
}

case_lint_arch__layer() {
  # An adapter that uses another adapter of its context.
  local module
  module=$(backend_module)
  put backend/internal/facility/internal/adapter/http/rows.go << EOF
package http

import _ "$module/internal/facility/internal/adapter/postgres"
EOF
  fails 'arch/layer'
  drop backend/internal/facility/internal/adapter/http/rows.go
  passes
}

case_lint_arch__domain_purity() {
  put_core_context
  put backend/internal/selftest/internal/domain/home.go << 'EOF'
package domain

import "os"

// Home is a fixture.
func Home() string { return os.Getenv("HOME") }
EOF
  fails 'arch/domain-purity'
  put backend/internal/selftest/internal/domain/home.go << 'EOF'
package domain

// Home is a fixture.
func Home(getenv func(string) string) string { return getenv("HOME") }
EOF
  passes
}

case_lint_arch__domain_clock() {
  put_core_context
  put backend/internal/selftest/internal/domain/late.go << 'EOF'
package domain

import "time"

// Late is a fixture.
func Late(start time.Time) bool { return time.Now().After(start) }
EOF
  fails 'arch/domain-clock'
  put backend/internal/selftest/internal/domain/late.go << 'EOF'
package domain

import "time"

// Late is a fixture.
func Late(start, now time.Time) bool { return now.After(start) }
EOF
  passes
}

case_lint_arch__route_registration() {
  # A use case that registers a route by hand, outside the API document.
  put backend/internal/facility/internal/app/export.go << 'EOF'
package app

import "net/http"

func register() {
    http.HandleFunc("/rooms/export", nil)
}
EOF
  fails 'arch/route-registration'
  drop backend/internal/facility/internal/app/export.go
  passes
}

case_lint_arch__schema_reference() {
  # A query that names the schema of its own context.
  local queries=backend/internal/facility/internal/adapter/postgres/queries/count.sql
  put "$queries" << 'EOF'
-- name: CountRooms :one
SELECT count(*) FROM facility.rooms;
EOF
  fails 'arch/schema-reference'
  put "$queries" << 'EOF'
-- name: CountRooms :one
SELECT count(*) FROM rooms;
EOF
  passes
}

case_lint_arch__glossary() {
  # An exported type that the glossary of Facility does not explain.
  put backend/internal/facility/internal/app/hold.go << 'EOF'
package app

// Hold keeps a room for a while.
type Hold struct{}
EOF
  fails 'arch/glossary'
  drop backend/internal/facility/internal/app/hold.go
  passes
}

case_test_commit_msg() {
  # A script that accepts every subject must fail its tests.
  put tools/scripts/commit-msg.sh << 'EOF'
#!/usr/bin/env bash
exit 0
EOF
  fails 'commit-msg/test'
  run git checkout -q -- tools/scripts/commit-msg.sh
  passes
}

case_lint_links() {
  put notes.md << 'EOF'
# Notes

See [the guide](missing-guide.md).
EOF
  fails 'missing-guide.md'
  put notes.md << 'EOF'
# Notes

See [the license](LICENSE).
EOF
  passes
}

# Go files outside a module are not checked, so these live in the tools module.
# This file is indented with spaces, so gofmt turns the fixtures into Go style.
put_go() {
  put "$1"
  run gofmt -w "$1"
}

good_go() {
  put_go tools/sample/sample.go << 'EOF'
// Package sample is a fixture of the self-test.
package sample

// Add returns the sum of a and b.
func Add(a, b int) int {
    return a + b
}
EOF
  put_go tools/sample/sample_test.go << 'EOF'
package sample

import "testing"

func TestAdd(t *testing.T) {
    if got := Add(1, 2); got != 3 {
        t.Fatalf("Add(1, 2) = %d, want 3", got)
    }
}
EOF
}

case_fmt_check_go() {
  good_go
  put tools/sample/sample.go << 'EOF'
// Package sample is a fixture of the self-test.
package sample

// Add returns the sum of a and b.
func Add(a,b int) int { return a+b }
EOF
  fails 'sample.go'
  good_go
  passes
}

case_lint_go() {
  good_go
  put_go tools/sample/print.go << 'EOF'
package sample

import "fmt"

// Print writes the sum of a and b.
func Print(a, b int) {
    fmt.Println(Add(a, b))
}
EOF
  fails 'forbidigo'
  drop tools/sample/print.go
  passes
}

case_test_go() {
  good_go
  put_go tools/sample/fail_test.go << 'EOF'
package sample

import "testing"

func TestFails(t *testing.T) {
    t.Fatal("the self-test expects this test to fail")
}
EOF
  fails 'TestFails'
  drop tools/sample/fail_test.go
  passes
}

good_workflow() {
  put .github/workflows/selftest.yml << 'EOF'
name: selftest
on:
  push:
    branches: [main]
permissions: {}
jobs:
  check:
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    steps:
      - run: echo ok
EOF
}

case_lint_actions() {
  put .github/workflows/selftest.yml << 'EOF'
name: selftest
on:
  push:
    branches: [main]
permissions: {}
jobs:
  check:
    timeout-minutes: 5
    steps:
      - run: echo ok
EOF
  fails 'syntax-check'
  good_workflow
  passes
}

case_lint_actions_security() {
  put .github/workflows/selftest.yml << 'EOF'
name: selftest
on:
  issues:
    types: [opened]
permissions: {}
jobs:
  check:
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    steps:
      - run: echo "${{ github.event.issue.title }}"
EOF
  fails 'template-injection'
  good_workflow
  passes
}

case_lint_actions_policy() {
  put .github/workflows/selftest.yml << 'EOF'
name: selftest
on:
  push:
    branches: [main]
permissions: {}
jobs:
  check:
    runs-on: ubuntu-24.04
    steps:
      - run: echo ok
EOF
  fails 'job_timeout_minutes_is_required'
  good_workflow
  passes
}

case_lint_actions_pins() {
  put .github/workflows/selftest.yml << 'EOF'
name: selftest
on:
  push:
    branches: [main]
permissions: {}
jobs:
  check:
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@v5
        with:
          persist-credentials: false
EOF
  fails 'actions/checkout@v5'
  good_workflow
  passes
}

case_scan_config__dockerfile() {
  put Dockerfile 'FROM alpine:3.22'
  fails 'DS-0002'
  put Dockerfile << 'EOF'
FROM alpine:3.22
USER 65534
EOF
  passes
}

# The Dockerfile of the backend, without its USER, runs as root.
case_scan_config__backend() {
  local dockerfile
  dockerfile=$(grep -v '^USER ' "$COPY/backend/Dockerfile")
  put backend/Dockerfile "$dockerfile"
  fails 'DS-0002'
  run git checkout -- backend/Dockerfile
  passes
}

# Base images pinned by digest. The Dockerfile of the backend, with the
# digests taken out, fails.
case_lint_image_pins() {
  local dockerfile
  dockerfile=$(sed 's/@sha256:[0-9a-f]*//' "$COPY/backend/Dockerfile")
  put backend/Dockerfile "$dockerfile"
  fails '[image-pins] the base image docker.io/library/golang:'
  prints '[image-pins] the base image gcr.io/distroless/'
  run git checkout -- backend/Dockerfile
  passes
  # An earlier stage, scratch, and a build argument are not images to pin.
  put Dockerfile << 'EOF'
FROM alpine:3.22@sha256:0000000000000000000000000000000000000000000000000000000000000000 AS base
FROM base AS build
FROM scratch
ARG IMAGE=base
FROM ${IMAGE}
EOF
  passes
}

case_scan_config__terraform() {
  put infra/modules/sample/main.tf << 'EOF'
resource "aws_security_group" "ssh" {
  name        = "ssh"
  description = "SSH from anywhere"

  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
EOF
  fails 'AWS-0107'
  put infra/modules/sample/main.tf << 'EOF'
variable "name" {
  description = "The name of the sample."
  type        = string
}
EOF
  passes
}

# A module that passes every rule of tflint, and its README.
good_module() {
  put infra/modules/sample/versions.tf << 'EOF'
terraform {
  required_version = ">= 1.15"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
EOF
  put infra/modules/sample/variables.tf << 'EOF'
variable "ami" {
  description = "The AMI of the instance."
  type        = string
}
EOF
  put infra/modules/sample/outputs.tf << 'EOF'
output "id" {
  description = "The ID of the instance."
  value       = aws_instance.web.id
}
EOF
  put infra/modules/sample/main.tf << 'EOF'
resource "aws_instance" "web" {
  ami           = var.ami
  instance_type = "t3.micro"
}
EOF
}

case_fmt_check_terraform() {
  good_module
  put infra/modules/sample/variables.tf << 'EOF'
variable "ami" {
description = "The AMI of the instance."
type = string
}
EOF
  fails 'infra/modules/sample/variables.tf:1: [terraform fmt]'
  good_module
  passes
}

case_fmt_check_terragrunt() {
  put infra/live/stg/sample/terragrunt.hcl << 'EOF'
terraform {
    source = "../../../modules/sample"
}
EOF
  fails 'infra/live/stg/sample/terragrunt.hcl:1: [terragrunt hcl fmt]'
  put infra/live/stg/sample/terragrunt.hcl << 'EOF'
terraform {
  source = "../../../modules/sample"
}
EOF
  passes
}

case_lint_terraform() {
  good_module
  put infra/modules/sample/main.tf << 'EOF'
resource "aws_instance" "web" {
  ami           = var.ami
  instance_type = "t1.2xlarge"
}
EOF
  fails 'aws_instance_invalid_type'
  good_module
  passes
}

case_lint_terraform_docs() {
  good_module
  put infra/modules/sample/README.md << 'EOF'
# Sample

<!-- BEGIN_TF_DOCS -->
<!-- END_TF_DOCS -->
EOF
  fails 'infra/modules/sample/README.md:1: [terraform-docs]'
  run "$make_cmd" --no-print-directory gen-terraform-docs
  passes
}

# A check-all target, cheap and offline for a module without providers.
case_validate_terraform() {
  put infra/modules/plain/main.tf << 'EOF'
output "name" {
  description = "The name."
  value       = var.name
}
EOF
  fails 'Reference to undeclared input variable'
  put infra/modules/plain/variables.tf << 'EOF'
variable "name" {
  description = "The name."
  type        = string
}
EOF
  passes
}

case_lint_sql() {
  put db/schema.sql << 'EOF'
CREATE TABLE rooms (
    id uuid DEFAULT uuidv7() PRIMARY KEY,
    name varchar(100) NOT NULL
);
EOF
  fails 'db/schema.sql:3: [prefer-text-field]'
  put db/schema.sql << 'EOF'
CREATE TABLE rooms (
    id uuid DEFAULT uuidv7() PRIMARY KEY,
    name text NOT NULL
);

CREATE INDEX rooms_name_idx ON rooms (name);
EOF
  passes
}

# The checks below are in check-all. They use the PostgreSQL of `make db-up`
# (or start it), and work in scratch databases of their own.
facility_postgres=backend/internal/facility/internal/adapter/postgres

# A table of rooms with an id and the given columns.
rooms_schema() {
  local columns='' column
  for column in "$@"; do
    columns="$columns    $column,"$'\n'
  done
  put "$facility_postgres/schema.sql" << EOF
CREATE TABLE rooms (
    id uuid NOT NULL,
${columns}    CONSTRAINT rooms_pkey PRIMARY KEY (id)
);
EOF
}

case_lint_sql_plan() {
  rooms_schema 'capacity integer NOT NULL'
  run git add "$facility_postgres/schema.sql"
  commit 'make the schema of the base small'
  passes CHECK_BASE=HEAD
  rooms_schema 'capacity integer NOT NULL' 'floor integer'
  passes CHECK_BASE=HEAD
  rooms_schema 'capacity integer NOT NULL' 'floor integer NOT NULL'
  fails '.cache/sql-plan/facility.sql:1: [adding-required-field]' CHECK_BASE=HEAD
  prints "$facility_postgres/schema.sql"
  rooms_schema 'capacity bigint NOT NULL'
  fails '[changing-column-type]' CHECK_BASE=HEAD
  # A dangerous statement that a person reviewed is let through by a line in
  # the allowance file of the context, and only while the plan holds it.
  rooms_schema 'capacity integer NOT NULL' 'floor integer NOT NULL'
  put "$facility_postgres/schema-changes.allow" 'adding-required-field | ALTER TABLE rooms ADD COLUMN floor integer NOT NULL | the table is empty in every environment'
  passes CHECK_BASE=HEAD
  put "$facility_postgres/schema-changes.allow" 'adding-required-field | ALTER TABLE rooms ADD COLUMN floor integer NOT NULL'
  fails '[sql-plan/allowance-format]' CHECK_BASE=HEAD
  rooms_schema 'capacity integer NOT NULL' 'floor integer'
  put "$facility_postgres/schema-changes.allow" 'adding-required-field | ALTER TABLE rooms ADD COLUMN floor integer NOT NULL | the table is empty in every environment'
  fails '[sql-plan/unused-allowance]' CHECK_BASE=HEAD
  drop "$facility_postgres/schema-changes.allow"
  # Without a base (no origin/main in the copy), nothing is compared.
  passes
}

case_lint_sqlc() {
  local queries="$facility_postgres/queries/selftest.sql"
  put "$queries" << 'EOF'
-- name: CountLargeRooms :one
SELECT count(*) FROM rooms WHERE capacity > 'many';
EOF
  fails "$queries:1: [sqlc/db-prepare] CountLargeRooms:"
  put "$queries" << 'EOF'
-- name: ClearRooms :exec
DELETE FROM rooms;
EOF
  fails "$queries:1: [no-unfiltered-write] ClearRooms:"
  put "$queries" << 'EOF'
-- name: CountLargeRooms :one
SELECT count(*) FROM rooms WHERE capacity > sqlc.arg(capacity);
EOF
  passes
}

case_lint_db_docs() {
  printf '\nCREATE TABLE seats (id uuid PRIMARY KEY);\n' >> "$COPY/$facility_postgres/schema.sql"
  fails '[tbls/requireTableComment] facility.seats: table comment required.'
  prints '[tbls/requireColumnComment] facility.seats.id: column comment required.'
  printf "COMMENT ON TABLE seats IS 'Seats.';\nCOMMENT ON COLUMN seats.id IS 'The id.';\n" \
    >> "$COPY/$facility_postgres/schema.sql"
  fails 'docs/generated/db/facility/README.md:1: [gen/stale]'
  run "$make_cmd" --no-print-directory gen-db-docs
  passes
}

good_openapi() {
  put contracts/http/facility.openapi.yaml << 'EOF'
openapi: 3.1.0
info:
  title: Facility API
  version: 1.0.0
  description: Meeting rooms.
  license:
    name: MIT-0
    identifier: MIT-0
servers:
  - url: https://facility.internal
security:
  - bearer: []
tags:
  - name: rooms
    description: Meeting rooms.
paths:
  /rooms/{roomId}:
    get:
      operationId: getRoom
      summary: Get a room
      tags:
        - rooms
      parameters:
        - name: roomId
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: The room.
          content:
            application/json:
              schema:
                type: object
        "404":
          description: There is no such room.
          content:
            application/problem+json:
              schema:
                type: object
components:
  securitySchemes:
    bearer:
      type: http
      scheme: bearer
EOF
}

case_lint_contracts__openapi() {
  good_openapi
  run sed -i.orig -e 's|application/problem+json|application/json|' contracts/http/facility.openapi.yaml
  fails 'rule/problem-details-only'
  good_openapi
  run sed -i.orig -e '/operationId: getRoom/d' contracts/http/facility.openapi.yaml
  fails 'operation-operationId'
  drop contracts/http/facility.openapi.yaml.orig
  good_openapi
  passes
}

# A small API document. $1: info.version. $2: the request property floor, as
# "optional", "required", or nothing. $3: "no-name" leaves the response
# property name out.
sample_openapi() {
  local floor='' required='[name]' returned='[id, name]' name="
                  name:
                    type: string"
  case "${2:-}" in
    optional | required)
      floor="
                floor:
                  type: integer"
      ;;
  esac
  if [ "${2:-}" = required ]; then
    required='[name, floor]'
  fi
  if [ "${3:-}" = no-name ]; then
    returned='[id]'
    name=''
  fi
  put contracts/http/sample.openapi.yaml << EOF
openapi: 3.1.0
info:
  title: Sample API
  version: $1
paths:
  /rooms:
    post:
      operationId: registerRoom
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: $required
              properties:
                name:
                  type: string$floor
      responses:
        "201":
          description: The room.
          content:
            application/json:
              schema:
                type: object
                required: $returned
                properties:
                  id:
                    type: string$name
EOF
}

case_lint_contracts_breaking() {
  sample_openapi 0.1.0
  # A document that is new since the base has nothing to compare with.
  passes CHECK_BASE=HEAD
  run git add contracts/http/sample.openapi.yaml
  commit 'add the sample API'
  sample_openapi 0.1.0 '' no-name
  fails 'contracts/http/sample.openapi.yaml:1: [response-required-property-removed] POST /rooms:' CHECK_BASE=HEAD
  sample_openapi 0.1.0 required
  fails 'contracts/http/sample.openapi.yaml:19: [new-required-request-property] POST /rooms:' CHECK_BASE=HEAD
  prints 'docs/guides/api-changes.md'
  # A patch version does not allow a breaking change; below 1.0, a minor one does.
  sample_openapi 0.1.1 required
  fails 'new-required-request-property' CHECK_BASE=HEAD
  sample_openapi 0.2.0 required
  passes CHECK_BASE=HEAD
  sample_openapi 0.1.0 optional
  passes CHECK_BASE=HEAD
  # Without a base (no origin/main in the copy), nothing is compared.
  sample_openapi 0.1.0 required
  passes
}

good_asyncapi() {
  put contracts/events/reservation.asyncapi.yaml << 'EOF'
asyncapi: 3.0.0
info:
  title: Reservation events
  version: 1.0.0
  description: The events that the reservation context publishes.
  license:
    name: MIT-0
    url: https://spdx.org/licenses/MIT-0.html
defaultContentType: application/cloudevents+json
channels:
  reservationConfirmed:
    address: reservation.confirmed
    messages:
      reservationConfirmed:
        $ref: "#/components/messages/ReservationConfirmed"
operations:
  sendReservationConfirmed:
    action: send
    channel:
      $ref: "#/channels/reservationConfirmed"
    messages:
      - $ref: "#/channels/reservationConfirmed/messages/reservationConfirmed"
components:
  messages:
    ReservationConfirmed:
      name: ReservationConfirmed
      payload:
        type: object
        required: [reservationId]
        properties:
          reservationId:
            type: string
            format: uuid
EOF
}

case_lint_contracts__asyncapi() {
  good_asyncapi
  # AsyncAPI 3.0 knows only send and receive.
  run sed -i.orig -e 's|action: send|action: publish|' contracts/events/reservation.asyncapi.yaml
  fails 'contracts/events/reservation.asyncapi.yaml:18: [struct]'
  drop contracts/events/reservation.asyncapi.yaml.orig
  good_asyncapi
  passes
}

# The event schemas of Reservation, which pass as they are.
events_dir=contracts/events/reservation
confirmed=$events_dir/ReservationConfirmed.v1.json
cancelled=$events_dir/ReservationCancelled.v1.json

case_lint_events() {
  # A property without a description.
  run sed -i.orig -e '/"description": "The number of people/d' "$confirmed"
  fails "$confirmed:28: [events/description] the property attendees at /properties/attendees has no description"
  run git checkout -- "$confirmed"
  drop "$confirmed.orig"
  passes
}

case_lint_events__file_name() {
  # A name that is not in UpperCamelCase. (One that differs from an existing
  # file only in case would be that file on macOS.)
  run cp "$cancelled" "$events_dir/reservation-moved.v1.json"
  fails "$events_dir/reservation-moved.v1.json:1: [events/file-name]"
  drop "$events_dir/reservation-moved.v1.json"
  passes
}

case_lint_events__compile() {
  run sed -i.orig -e 's/"type": "integer"/"type": "integr"/' "$confirmed"
  fails "$confirmed:30: [events/compile] the schema breaks the rules of JSON Schema 2020-12 at /properties/attendees/type"
  run git checkout -- "$confirmed"
  drop "$confirmed.orig"
  passes
}

case_lint_events__examples() {
  # An example that breaks a bound, then one that breaks a format.
  run sed -i.orig -e 's/"attendees": 4/"attendees": 0/' "$confirmed"
  fails "$confirmed:46: [events/examples] example 0 does not match the schema at /attendees"
  run git checkout -- "$confirmed"
  run sed -i.orig -e 's/"startsAt": "2026-11-20T01:00:00Z"/"startsAt": "tomorrow"/' "$confirmed"
  fails "$confirmed:44: [events/examples] example 0 does not match the schema at /startsAt"
  run git checkout -- "$confirmed"
  drop "$confirmed.orig"
  passes
}

case_lint_events__closed() {
  run sed -i.orig -e 's/"type": "object",/"type": "object", "additionalProperties": false,/' "$confirmed"
  fails "$confirmed:5: [events/closed]"
  run git checkout -- "$confirmed"
  drop "$confirmed.orig"
  passes
}

case_lint_events__asyncapi() {
  # A schema that no message refers to: a copy of ReservationCancelled under
  # another name.
  local moved=$events_dir/ReservationMoved.v1.json
  sed -e 's/"title": "ReservationCancelled"/"title": "ReservationMoved"/' "$COPY/$cancelled" | put "$moved"
  fails "$moved:1: [events/asyncapi] no message of contracts/events/reservation.asyncapi.yaml is named reservation.ReservationMoved.v1"
  drop "$moved"
  # A message whose payload refers to another schema.
  run sed -i.orig -e 's|: ./reservation/ReservationCancelled.v1.json|: ./reservation/ReservationConfirmed.v1.json|' \
    contracts/events/reservation.asyncapi.yaml
  fails 'contracts/events/reservation.asyncapi.yaml:73: [events/asyncapi] the message reservation.ReservationCancelled.v1 does not refer'
  run git checkout -- contracts/events/reservation.asyncapi.yaml
  drop contracts/events/reservation.asyncapi.yaml.orig
  passes
}

# A change to the AsyncAPI document alone checks the schemas, which depend on it.
case_lint_events__scope() {
  run sed -i.orig -e 's|name: reservation.ReservationCancelled.v1|name: reservation.ReservationCanceled.v1|' \
    contracts/events/reservation.asyncapi.yaml
  drop contracts/events/reservation.asyncapi.yaml.orig
  fails "$cancelled:1: [events/asyncapi]" CHECK_SCOPE=changed CHECK_BASE=HEAD
  run git checkout -- contracts/events/reservation.asyncapi.yaml
  passes CHECK_SCOPE=changed CHECK_BASE=HEAD
}

# A small event schema. $1: the required properties. $2: the type of the
# property floor, or "none" to leave it out. $3: more properties, each after a
# comma. $4: the description of the event.
sample_event() {
  local floor=''
  if [ "$2" != none ]; then
    floor=",
    \"floor\": { \"description\": \"The floor of the room.\", \"type\": \"$2\" }"
  fi
  put contracts/events/sample/RoomCleaned.v1.json << EOF
{
  "\$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "RoomCleaned",
  "description": "${4:-A room was cleaned.}",
  "type": "object",
  "required": $1,
  "properties": {
    "roomId": { "description": "The id of the room.", "type": "string", "format": "uuid" }$floor${3:-}
  },
  "examples": [{ "roomId": "0193a6f0-1a2b-7c3d-8e4f-5a6b7c8d9e0f", "floor": 2 }]
}
EOF
}

case_lint_events_breaking() {
  local sample=contracts/events/sample/RoomCleaned.v1.json
  sample_event '["roomId", "floor"]' integer
  # A schema that is new since the base has nothing to compare with.
  passes CHECK_BASE=HEAD
  run git add "$sample"
  commit 'add the sample event'
  passes CHECK_BASE=HEAD
  sample_event '["roomId"]' none
  fails "$sample:7: [events/property-removed] the property floor at /properties/floor was removed since" CHECK_BASE=HEAD
  prints 'publish the change as a new version, RoomCleaned.v2.json, and keep this file as it was'
  sample_event '["roomId"]' integer
  fails "$sample:6: [events/required-removed] /required no longer names \"floor\"" CHECK_BASE=HEAD
  sample_event '["roomId", "floor"]' number
  fails "$sample:9: [events/keyword-changed] /properties/floor/type changed from \"integer\" to \"number\"" CHECK_BASE=HEAD
  # An added optional property and a changed description keep the promise.
  sample_event '["roomId", "floor"]' integer ',
    "wing": { "description": "The wing of the building.", "type": "string" }' 'A room was cleaned, and is ready again.'
  passes CHECK_BASE=HEAD
  # Without a base (no origin/main in the copy), nothing is compared.
  sample_event '["roomId"]' none
  passes
}

case_lint_events_breaking__deleted() {
  drop "$cancelled"
  fails "$cancelled:1: [events/schema-deleted] the schema of reservation.ReservationCancelled.v1 existed at" CHECK_BASE=HEAD
  # A renamed schema is a deleted one.
  run git checkout -- "$cancelled"
  run git mv "$cancelled" "$events_dir/ReservationCanceled.v1.json"
  fails "$cancelled:1: [events/schema-deleted]" CHECK_BASE=HEAD
  run git mv "$events_dir/ReservationCanceled.v1.json" "$cancelled"
  passes CHECK_BASE=HEAD
}

good_frontend() {
  put frontend/src/lib/format.ts << 'EOF'
export function formatCapacity(capacity: number): string {
  return `${String(capacity)} people`;
}
EOF
  put frontend/src/features/rooms/capacity.ts << 'EOF'
import { formatCapacity } from "@/lib/format";

export function capacityLabel(capacity: number): string {
  return formatCapacity(capacity);
}
EOF
  put frontend/src/app/page.tsx << 'EOF'
import { capacityLabel } from "@/features/rooms/capacity";

export default function Page() {
  return <p>{capacityLabel(8)}</p>;
}
EOF
}

case_lint_frontend() {
  good_frontend
  # A feature never uses another feature.
  put frontend/src/features/reservations/limit.ts << 'EOF'
import { capacityLabel } from "@/features/rooms/capacity";

export const limit = capacityLabel(4);
EOF
  fails 'frontend/src/features/reservations/limit.ts:1: [boundaries/dependencies]'
  drop frontend/src/features/reservations
  passes
}

case_lint_frontend__server_only() {
  good_frontend
  put frontend/src/features/rooms/server/rooms.ts << 'EOF'
export function loadRooms(): string[] {
  return [];
}
EOF
  fails 'frontend/src/features/rooms/server/rooms.ts:1: [no-restricted-syntax]'
  put frontend/src/features/rooms/server/rooms.ts << 'EOF'
import "server-only";

export function loadRooms(): string[] {
  return [];
}
EOF
  put frontend/src/app/rooms.tsx << 'EOF'
"use client";
import { loadRooms } from "@/features/rooms/server/rooms";

export function Rooms() {
  return <p>{loadRooms().join(", ")}</p>;
}
EOF
  fails 'frontend/src/app/rooms.tsx:2: [no-restricted-syntax]'
  drop frontend/src/app/rooms.tsx
  passes
}

case_lint_frontend_types() {
  good_frontend
  put frontend/src/lib/format.ts << 'EOF'
export function formatCapacity(capacity: number): string {
  return capacity;
}
EOF
  fails 'frontend/src/lib/format.ts:2: [TS2322]'
  good_frontend
  passes
}

case_lint_frontend_unused() {
  good_frontend
  put frontend/src/lib/unused.ts 'export const unused = 1;'
  fails 'frontend/src/lib/unused.ts:1: [knip/files]'
  drop frontend/src/lib/unused.ts
  passes
}

# The scope of every check (tools/scripts/files.sh), through lint-shell.
bad_shell() {
  put "$1" << 'EOF'
#!/usr/bin/env bash
echo $1
EOF
}

case_scope__check_files() {
  TARGET=lint-shell
  bad_shell a.sh
  bad_shell b.sh
  fails 'a.sh' CHECK_FILES=a.sh
  omits 'b.sh'
  passes CHECK_FILES=LICENSE
}

case_scope__staged() {
  TARGET=lint-shell
  bad_shell staged.sh
  run git add staged.sh
  bad_shell untracked.sh
  fails 'staged.sh' CHECK_SCOPE=staged
  omits 'untracked.sh'
  run git reset --quiet staged.sh
  passes CHECK_SCOPE=staged
}

# The license rule: a dependency that ships under a forbidden license fails,
# and one under a weak copyleft license passes. Trivy reads the license of an
# npm package from node_modules.
npm_project_with_license() {
  put sample/package.json '{"name":"sample","version":"1.0.0","dependencies":{"somepkg":"1.0.0"}}'
  put sample/package-lock.json << 'EOF'
{
  "name": "sample",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": { "name": "sample", "version": "1.0.0", "dependencies": { "somepkg": "1.0.0" } },
    "node_modules/somepkg": {
      "version": "1.0.0",
      "resolved": "https://registry.npmjs.org/somepkg/-/somepkg-1.0.0.tgz",
      "integrity": "sha512-aaaa"
    }
  }
}
EOF
  put sample/node_modules/somepkg/package.json "{\"name\":\"somepkg\",\"version\":\"1.0.0\",\"license\":\"$1\"}"
}

case_scan_deps__license() {
  npm_project_with_license GPL-3.0-only
  fails 'somepkg is under GPL-3.0-only'
  npm_project_with_license LGPL-3.0-or-later
  passes
}

# The targets that build the images and start the services would replace the
# containers of the repository itself, because every copy shares its Docker
# daemon and its Compose project. So these cases put a stand-in for the Docker
# CLI first on PATH: PostgreSQL is ready (db-server), and every other command
# fails and names itself. A change outside the sources of the images skips the
# target without a call to Docker; a change to them runs the target, which
# then fails at its first call.
fake_docker() {
  mkdir -p "$COPY.bin" || die "cannot create $COPY.bin"
  cat > "$COPY.bin/docker" << 'EOF'
#!/usr/bin/env bash
case "$*" in
  *pg_isready*) exit 0 ;;
esac
echo "fake docker: $*" >&2
exit 1
EOF
  chmod +x "$COPY.bin/docker"
  printf 'PATH=%s:%s\n' "$COPY.bin" "$PATH"
}

case_smoke() {
  local path
  path=$(fake_docker)
  put docs/notes.md '# Notes'
  passes CHECK_SCOPE=changed CHECK_BASE=HEAD "$path"
  omits 'fake docker'
  put backend/notes.txt 'A change to the backend.'
  fails 'fake docker: compose config' CHECK_SCOPE=changed CHECK_BASE=HEAD "$path"
}

case_scan_images() {
  local path
  path=$(fake_docker)
  put docs/notes.md '# Notes'
  passes CHECK_SCOPE=changed CHECK_BASE=HEAD "$path"
  omits 'fake docker'
  printf '# A change to the system.\n' >> "$COPY/compose.yaml"
  fails 'fake docker: compose config' CHECK_SCOPE=changed CHECK_BASE=HEAD "$path"
}

# A staged change to a tool pin or to the checks puts every file in scope.
case_scope__pins() {
  TARGET=lint-shell
  bad_shell untracked.sh
  passes CHECK_SCOPE=staged
  printf '\n# A comment.\n' >> "$COPY/mise.toml"
  run git add mise.toml
  fails 'untracked.sh' CHECK_SCOPE=staged
}

case_scope__changed() {
  TARGET=lint-shell
  bad_shell old.sh
  run git add old.sh
  commit 'add a file before the base'
  run git rev-parse HEAD
  local base
  base=$(cat "$OUT")
  bad_shell committed.sh
  run git add committed.sh
  commit 'change a file after the base'
  bad_shell untracked.sh
  fails 'committed.sh' CHECK_SCOPE=changed CHECK_BASE="$base"
  prints 'untracked.sh'
  omits 'old.sh'
  drop untracked.sh
  passes CHECK_SCOPE=changed CHECK_BASE=HEAD
}

case_scope__lintignore() {
  TARGET=lint-shell
  bad_shell vendor/tool.sh
  fails 'vendor/tool.sh'
  printf 'vendor\n' >> "$COPY/.lintignore"
  passes
}

# ---------------------------------------------------------------------------
# The runner.

# case_lint_secrets__staged -> lint-secrets, and lint-secrets (staged).
target_of() {
  local name=${1#case_}
  name=${name%%__*}
  printf '%s\n' "${name//_/-}"
}

title_of() {
  local name=${1#case_} variant=
  case "$name" in
    *__*)
      variant=${name#*__}
      variant=" (${variant//_/-})"
      ;;
  esac
  printf '%s%s\n' "$(target_of "$1")" "$variant"
}

# The cases in the order of this file.
all_cases=$(grep -oE '^case_[a-z0-9_]+\(\)' "$self" | tr -d '()')

# Every check target needs a case.
missing=0
for target in "$@"; do
  found=false
  for fn in $all_cases; do
    if [ "$(target_of "$fn")" = "$target" ]; then
      found=true
      break
    fi
  done
  if [ "$found" = false ]; then
    printf '%-5s %-34s no case: add case_%s to tools/selftest/selftest.sh\n' FAIL "$target" "${target//-/_}"
    missing=$((missing + 1))
  fi
done

work_tmp=$(mktemp -d "${TMPDIR:-/tmp}/selftest.XXXXXX") || exit 2
work=$(cd "$work_tmp" && pwd -P) || exit 2
# mise gives the copies the pinned tools, as it does the repository.
export MISE_TRUSTED_CONFIG_PATHS="${MISE_TRUSTED_CONFIG_PATHS:+$MISE_TRUSTED_CONFIG_PATHS:}$work"
cleanup() {
  local running
  running=$(jobs -p)
  if [ -n "$running" ]; then
    # shellcheck disable=SC2086 # one process ID per word
    kill $running 2> /dev/null
    wait
  fi
  rm -rf "$work"
}
trap cleanup EXIT

# The base copy: what Git would see in the working tree, committed.
base="$work/base"
mkdir -p "$base"
git ls-files -z --cached --others --exclude-standard \
  | while IFS= read -r -d '' file; do
    if [ -f "$file" ]; then
      printf '%s\0' "$file"
    fi
  done \
  | tar --null -T - -cf - | (cd "$base" && tar -xf -) || die "cannot copy the repository"
for dir in node_modules .venv */node_modules; do
  if [ -d "$dir" ]; then
    ln -s "$repo/$dir" "$base/$dir"
  fi
done

# From here on, the Git configuration of the user does not apply: the copies
# behave the same on every machine.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=selftest GIT_AUTHOR_EMAIL=selftest@example.invalid
export GIT_COMMITTER_NAME=selftest GIT_COMMITTER_EMAIL=selftest@example.invalid
(
  cd "$base" \
    && git init --quiet \
    && mkdir -p .git/info \
    && printf 'node_modules\n.venv\n' >> .git/info/exclude \
    && git add --all \
    && git commit --quiet --no-verify --message base
) > "$work/base.log" 2>&1 || {
  cat "$work/base.log"
  die "cannot create the base copy"
}

run_case() {
  local fn=$1 start
  start=$(date +%s)
  COPY="$work/$fn"
  OUT="$work/$fn.out"
  TARGET=$(target_of "$fn")
  LAST="the last command"
  cp -RP "$base" "$COPY" || die "cannot copy the base"
  ("$fn")
  local code=$?
  printf '%s\n' "$(($(date +%s) - start))" > "$work/$fn.time"
  return "$code"
}

passed=0
failed=0
print_result() {
  local fn=$1 code=$2 status=ok
  if [ "$code" -ne 0 ]; then
    status=FAIL
    failed=$((failed + 1))
  else
    passed=$((passed + 1))
  fi
  printf '%-5s %-34s %ss\n' "$status" "$(title_of "$fn")" "$(cat "$work/$fn.time" 2> /dev/null || echo '?')"
  if [ "$code" -ne 0 ] || [ "$verbose" = 1 ]; then
    sed 's/^/      /' "$work/$fn.log"
  fi
}

start=$(date +%s)
pids=()
names=()
for fn in $all_cases; do
  if [ -n "$only" ]; then
    case "$(title_of "$fn")" in
      *"$only"*) ;;
      *) continue ;;
    esac
  fi
  run_case "$fn" > "$work/$fn.log" 2>&1 &
  pids+=("$!")
  names+=("$fn")
  if [ "${#pids[@]}" -ge "$jobs" ]; then
    wait "${pids[0]}"
    print_result "${names[0]}" "$?"
    pids=("${pids[@]:1}")
    names=("${names[@]:1}")
  fi
done
while [ "${#pids[@]}" -gt 0 ]; do
  wait "${pids[0]}"
  print_result "${names[0]}" "$?"
  pids=("${pids[@]:1}")
  names=("${names[@]:1}")
done

summary="selftest: $passed passed, $failed failed in $(($(date +%s) - start))s"
if [ "$missing" -gt 0 ]; then
  summary="$summary; $missing check targets have no case"
fi
echo "$summary"
if [ "$failed" -gt 0 ] || [ "$missing" -gt 0 ]; then
  exit 1
fi
