#!/usr/bin/env bash
# The environments in infra/live/ (TPL-0015), through Terragrunt.
#
# Usage: infra.sh lint                 the shape of the tree: the same units
#                                      in every environment, a definition for
#                                      every unit, a unit for every context
#        infra.sh validate             `terraform validate` of every unit,
#                                      without an account
#        infra.sh rehearse             apply stg to AWS Emulator on this
#                                      machine, and plan prd against it
#        infra.sh lock                 write the checksums of the providers in
#                                      infra/live/providers.hcl to
#                                      providers.lock.hcl next to it
#        infra.sh bootstrap <env>      once for an account, with administrator
#                                      access: create the bucket of the state
#                                      and the roles of GitHub Actions
#        infra.sh plan <env>           plan every unit
#        infra.sh deploy <env>         push the images, prepare the database,
#                                      and apply every unit
#
# Run from the root of the repository. Terragrunt copies each unit into
# .cache/terragrunt/ and runs Terraform there, so the tree in infra/live/
# holds no working data. The providers come from the cache that every
# Terraform run of the repository shares (.cache/terraform/plugins).
#
# `bootstrap`, `plan`, and `deploy` reach AWS with the credentials of the
# environment: in a workflow, those of the role that the job assumed
# (TPL-0018). The provider of each unit refuses every account but the one in
# env.hcl. `deploy` refuses prd outside GitHub Actions: prd changes through
# the workflow, after a person approved the job.
#
# The order of a deployment:
#   1. the unit `registry`, then the images whose tag the registry lacks
#      (tools/scripts/images.sh);
#   2. the units `network`, `database`, and `platform`;
#   3. the task that prepares the database (db/): roles, passwords, and the
#      schema of every context. A schema change must therefore work with the
#      code that still runs (docs/guides/change-a-schema.md);
#   4. every unit. A service whose image tag did not change is left alone.
#
# `rehearse` does 1, 2, and 4 with the definitions of stg against AWS
# Emulator, without images and without the task: the emulator keeps what
# Terraform creates and runs no containers. It then plans prd there, which
# checks the values of prd/env.hcl against the validations of the modules.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

live=infra/live
pins=.cache/terraform/pins

export TG_TF_PATH=terraform
export TG_NON_INTERACTIVE=true
export TG_NO_TIPS=true
export TG_NO_COLOR=true
export TF_PLUGIN_CACHE_DIR="$PWD/.cache/terraform/plugins"
export TF_IN_AUTOMATION=1
export TF_INPUT=0

environments() {
  local file
  for file in "$live"/*/env.hcl; do
    [ -f "$file" ] || continue
    file=${file%/env.hcl}
    printf '%s\n' "${file##*/}"
  done
}

# The units of one run install their providers at the same time, and Terraform
# does not guard the shared cache against that. One init of the pinned
# providers, behind the lock of terraform-init.sh, fills the cache first;
# after it, the units only link what the cache holds.
fill_provider_cache() {
  local out
  pins_root
  if ! out=$(tools/scripts/terraform-init.sh "$pins" -backend=false -lockfile=readonly 2>&1); then
    printf '%s\n' "$out" >&2
    echo "infra: cannot install the providers of $live/providers.hcl; after a change to it, run \`make infra-lock\`" >&2
    exit 1
  fi
}

# A root module that holds the pinned providers and their lock file under the
# names that Terraform looks for.
pins_root() {
  mkdir -p "$pins"
  cp "$live/providers.hcl" "$pins/versions.tf"
  cp "$live/providers.lock.hcl" "$pins/.terraform.lock.hcl"
}

# run_all <mode> <environment> <Terraform command and its arguments>...
# runs the command in every unit of the environment, in the order of their
# dependencies. The working data of each mode is kept apart, so a validate without a
# backend and a run with one never share an initialized directory. Its
# directory must be named .terragrunt-cache: Terragrunt looks for units in the
# whole repository, and skips only directories of that name, so under another
# name it would take its own copies of the units for units.
run_all() {
  local mode="$1" environment="$2"
  shift 2
  TG_DOWNLOAD_DIR="$PWD/.cache/terragrunt/$mode/$environment/.terragrunt-cache" \
    terragrunt run --all --log-level error --working-dir "$live/$environment" -- "$@"
}

# What Terragrunt and Terraform printed, as findings: without the time and
# the level of each line, with paths from the root of the repository, and
# with the environment in front.
findings() {
  local rule="$1" environment="$2"
  sed -E -e '/^[[:space:]]*$/d' \
    -e 's/^[0-9:.]+ (ERROR|WARN|INFO|STDERR|STDOUT) +//' \
    -e "s|$PWD/||g" \
    -e "s|^|$live/$environment/env.hcl:1: [$rule] |"
}

# One environment stands for all: `lint` proves that they have the same
# units, and `terraform validate` reads no value of env.hcl. The values are
# checked by the rehearsal, which applies or plans every environment.
validate() {
  local environment log status=0
  environment=$(environments | head -n 1)
  fill_provider_cache
  log=$(mktemp)
  if ! INFRA_OFFLINE=1 run_all offline "$environment" validate -no-color > "$log" 2>&1; then
    findings 'terragrunt validate' "$environment" < "$log"
    status=1
  fi
  rm -f "$log"
  return "$status"
}

# The shape of the tree, which Terragrunt does not check. stg and prd must
# differ in the values of env.hcl alone (TPL-0015), so:
#   - every environment has the same units, and every terragrunt.hcl is the
#     same file;
#   - every unit has its definition in _units/, and every definition a unit;
#   - every env.hcl sets the same keys;
#   - the contexts of system.hcl are the commands of backend/cmd/ and the
#     units under contexts/.
lint() {
  local status=0 first="" reference="" environment unit file key
  # Each lookup lets grep read its whole input: with -q it would stop at
  # the first match, and a writer that is still writing would fail the
  # pipeline (pipefail), which would look like a key that is missing.
  finding() {
    printf '%s:1: [infra-live] %s\n' "$1" "$2"
    status=1
  }
  units_of() {
    find "$live/$1" -name terragrunt.hcl -not -path '*/.terragrunt-cache/*' \
      | sed -e "s|^$live/$1/||" -e 's|/terragrunt.hcl$||' | sort
  }
  keys_of() {
    sed -n -E 's/^  ([a-z_]+) += .*/\1/p' "$live/$1/env.hcl" | sort
  }

  for environment in $(environments); do
    if [ -z "$first" ]; then
      first=$environment
      reference=$(units_of "$first" | head -n 1)
      reference="$live/$first/$reference/terragrunt.hcl"
    fi
    for unit in $(units_of "$environment"); do
      file="$live/$environment/$unit/terragrunt.hcl"
      [ -f "$live/_units/$unit.hcl" ] \
        || finding "$file" "the unit has no definition: add $live/_units/$unit.hcl"
      cmp -s "$file" "$reference" \
        || finding "$file" "every terragrunt.hcl of the tree is the same file: copy $reference"
      [ -f "$live/$first/$unit/terragrunt.hcl" ] \
        || finding "$file" "$first has no unit $unit: every environment has the same units"
    done
    for unit in $(units_of "$first"); do
      [ -f "$live/$environment/$unit/terragrunt.hcl" ] \
        || finding "$live/$environment/env.hcl" "the unit $unit of $first is missing: copy $live/$first/$unit/"
    done
    for key in $(keys_of "$first"); do
      keys_of "$environment" | grep -x "$key" > /dev/null \
        || finding "$live/$environment/env.hcl" "the key $key of $live/$first/env.hcl is missing"
    done
    for key in $(keys_of "$environment"); do
      keys_of "$first" | grep -x "$key" > /dev/null \
        || finding "$live/$environment/env.hcl" "the key $key is not in $live/$first/env.hcl: every env.hcl sets the same keys"
    done
  done
  [ -n "$first" ] || return 0

  while IFS= read -r file; do
    unit=${file#"$live/_units/"}
    unit=${unit%.hcl}
    [ -f "$live/$first/$unit/terragrunt.hcl" ] \
      || finding "$file" "no environment has a unit $unit: add $live/<environment>/$unit/terragrunt.hcl, or remove the definition"
  done < <(find "$live/_units" -name '*.hcl' | sort)

  local listed commands
  listed=$(sed -n -E 's/^  contexts += \[(.*)\]$/\1/p' "$live/system.hcl" | tr -d '" ' | tr ',' '\n' | sed '/^$/d' | sort)
  commands=$(find backend/cmd -mindepth 1 -maxdepth 1 -type d 2> /dev/null | sed 's|.*/||' | sort)
  for key in $commands; do
    printf '%s\n' "$listed" | grep -x "$key" > /dev/null \
      || finding "$live/system.hcl" "the context $key of backend/cmd/ is not in \`contexts\`"
    [ -f "$live/_units/contexts/$key.hcl" ] \
      || finding "$live/system.hcl" "the context $key has no unit: add $live/_units/contexts/$key.hcl and $live/<environment>/contexts/$key/terragrunt.hcl"
  done
  for key in $listed; do
    printf '%s\n' "$commands" | grep -x "$key" > /dev/null \
      || finding "$live/system.hcl" "\`contexts\` names $key, which has no command in backend/cmd/"
  done
  return "$status"
}

# The four platforms of mise.toml: the laptops and the runners.
lock() {
  pins_root
  terraform -chdir="$pins" providers lock \
    -platform=linux_amd64 -platform=linux_arm64 -platform=darwin_arm64 -platform=darwin_amd64
  cp "$pins/.terraform.lock.hcl" "$live/providers.lock.hcl"
}

require_environment() {
  [ -f "$live/${1:-}/env.hcl" ] || {
    echo "infra: name an environment: $(environments | tr '\n' ' ')" >&2
    exit 2
  }
}

# run_unit <mode> <environment> <unit> <Terraform command and its arguments>...
run_unit() {
  local mode="$1" environment="$2" unit="$3"
  shift 3
  TG_DOWNLOAD_DIR="$PWD/.cache/terragrunt/$mode/$environment/.terragrunt-cache" \
    terragrunt run --log-level error --working-dir "$live/$environment/$unit" -- "$@"
}

# The Region of an environment, for the AWS CLI.
region_of() {
  sed -n -E 's/^  region += "(.*)"$/\1/p' "$live/$1/env.hcl"
}

# Steps 1, 2, and 4 of a deployment. Between them, `deploy` pushes the images
# and prepares the database.
apply_foundation() {
  local mode="$1" environment="$2" unit
  for unit in network database platform; do
    run_unit "$mode" "$environment" "$unit" apply -auto-approve -no-color
  done
}

# Runs the task of the unit `database` once in the cluster of the unit
# `platform`, waits until it stops, and fails unless its container exited
# with 0.
prepare_database() {
  local environment="$1" task cluster network arn code
  task=$(run_unit aws "$environment" database output -json db_task)
  cluster=$(run_unit aws "$environment" platform output -raw cluster_arn)
  network=$(jq -c '{awsvpcConfiguration: {subnets: .subnet_ids, securityGroups: [.security_group_id], assignPublicIp: "DISABLED"}}' <<< "$task")
  arn=$(aws ecs run-task --cluster "$cluster" --launch-type FARGATE \
    --task-definition "$(jq -r .task_definition_arn <<< "$task")" \
    --network-configuration "$network" --started-by infra.sh \
    --query 'tasks[0].taskArn' --output text)
  echo "infra: preparing the database: $arn"
  aws ecs wait tasks-stopped --cluster "$cluster" --tasks "$arn"
  code=$(aws ecs describe-tasks --cluster "$cluster" --tasks "$arn" \
    --query 'tasks[0].containers[0].exitCode' --output text)
  if [ "$code" != 0 ]; then
    # A task that never started its container, because the image or a secret
    # could not be read, has no exit code; ECS says why it stopped.
    aws ecs describe-tasks --cluster "$cluster" --tasks "$arn" \
      --query 'tasks[0].[stoppedReason, containers[0].reason]' --output text >&2
    echo "infra: the task that prepares the database failed (exit code $code); its output is in the log group /$environment/db" >&2
    exit 1
  fi
}

bootstrap() {
  local environment="$1"
  require_environment "$environment"
  fill_provider_cache
  TG_DOWNLOAD_DIR="$PWD/.cache/terragrunt/aws/$environment/.terragrunt-cache" \
    terragrunt backend bootstrap --log-level error --working-dir "$live/$environment/ci"
  # A person runs this, so Terraform shows its plan and asks before it
  # applies.
  TF_INPUT=1 TG_NON_INTERACTIVE=false run_unit aws "$environment" ci apply -no-color
}

plan() {
  local environment="$1"
  require_environment "$environment"
  fill_provider_cache
  run_all aws "$environment" plan -no-color -lock-timeout=300s
}

deploy() {
  local environment="$1" registry
  require_environment "$environment"
  if [ "$environment" = prd ] && [ "${GITHUB_ACTIONS:-}" != true ]; then
    echo "infra: prd is deployed by the workflow, after a person approved the job (TPL-0018); run \`make infra-plan ENV=prd\` to see what it would change" >&2
    exit 1
  fi
  AWS_REGION=$(region_of "$environment")
  export AWS_REGION
  fill_provider_cache

  run_unit aws "$environment" registry apply -auto-approve -no-color
  registry=$(run_unit aws "$environment" registry output -json repository_urls | jq -r '[.[]][0] | split("/")[0]')
  tools/scripts/images.sh push "$registry"

  apply_foundation aws "$environment"
  prepare_database "$environment"
  run_all aws "$environment" apply -no-color -lock-timeout=300s
}

domain_of() {
  sed -n -E 's/^  domain_name += "(.*)"$/\1/p' "$live/$1/env.hcl"
}

# What an account holds before its first deployment exists in the emulator
# only if the rehearsal puts it there. The hosted zone of the domain name of
# stg, once for the life of the emulator; the unit `web` takes its ID in
# place of the one in env.hcl (infra/live/_units/web.hcl).
emulator_hosted_zone() {
  local domain id
  domain=$(domain_of stg)
  id=$(aws route53 list-hosted-zones-by-name --dns-name "$domain." \
    --query "HostedZones[?Name=='$domain.'].Id | [0]" --output text)
  if [ -z "$id" ] || [ "$id" = None ]; then
    id=$(aws route53 create-hosted-zone --name "$domain" --caller-reference "rehearsal-$(date +%s)" \
      --query HostedZone.Id --output text)
  fi
  printf '%s\n' "${id##*/}"
}

# The emulator never issues a certificate, so the unit `web` could wait for
# one forever. The rehearsal requests a certificate for the domain name of
# stg itself, once for the life of the emulator, and the unit takes its ARN.
# The load balancer of the emulator accepts a certificate that is not issued.
emulator_certificate() {
  local region domain arn
  region=$(region_of stg)
  domain=$(domain_of stg)
  arn=$(aws acm list-certificates --region "$region" \
    --query "CertificateSummaryList[?DomainName=='$domain'].CertificateArn | [0]" --output text)
  if [ -z "$arn" ] || [ "$arn" = None ]; then
    arn=$(aws acm request-certificate --region "$region" --domain-name "$domain" \
      --validation-method DNS --query CertificateArn --output text)
  fi
  printf '%s\n' "$arn"
}

rehearse() {
  local log environment
  # The endpoint of every AWS service, the CA of the emulator, and
  # placeholder credentials. root.hcl refuses an endpoint that is not on this
  # machine, and no credentials of a real account stay in the environment.
  unset AWS_PROFILE AWS_SESSION_TOKEN AWS_ROLE_ARN AWS_WEB_IDENTITY_TOKEN_FILE
  eval "$(tools/scripts/aws.sh env)"
  export INFRA_EMULATOR=1
  # The emulator keeps the state in its S3, so the state lives exactly as
  # long as the resources that it describes; the bucket is created on the
  # first run after a start.
  export TG_BACKEND_BOOTSTRAP=true
  fill_provider_cache
  INFRA_EMULATOR_CERTIFICATE_ARN=$(emulator_certificate)
  INFRA_EMULATOR_HOSTED_ZONE_ID=$(emulator_hosted_zone)
  export INFRA_EMULATOR_CERTIFICATE_ARN INFRA_EMULATOR_HOSTED_ZONE_ID

  log=$(mktemp)
  if ! {
    run_unit emulator stg registry apply -auto-approve -no-color \
      && apply_foundation emulator stg \
      && run_all emulator stg apply -no-color -lock-timeout=300s
  } > "$log" 2>&1; then
    grep -E 'ERROR|STDERR|Error' "$log" | findings 'terragrunt apply' stg
    rm -f "$log"
    echo "infra: stg does not apply to AWS Emulator" >&2
    exit 1
  fi
  for environment in $(environments); do
    [ "$environment" != stg ] || continue
    if ! run_all emulator "$environment" plan -no-color -lock=false > "$log" 2>&1; then
      grep -E 'ERROR|STDERR|Error' "$log" | findings 'terragrunt plan' "$environment"
      rm -f "$log"
      echo "infra: $environment does not plan against AWS Emulator" >&2
      exit 1
    fi
  done
  rm -f "$log"
}

case "${1:-}" in
  lint)
    lint
    ;;
  validate)
    validate
    ;;
  rehearse)
    rehearse
    ;;
  lock)
    lock
    ;;
  bootstrap)
    bootstrap "${2:-}"
    ;;
  plan)
    plan "${2:-}"
    ;;
  deploy)
    deploy "${2:-}"
    ;;
  *)
    echo "usage: infra.sh lint|validate|rehearse|lock|bootstrap <env>|plan <env>|deploy <env>" >&2
    exit 2
    ;;
esac
