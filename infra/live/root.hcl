# What every unit of every environment shares (TPL-0015): where its state
# lives, which AWS account and Region its provider talks to, and which
# versions of the providers it installs. A unit is a directory with a
# terragrunt.hcl, <environment>/<unit>/; it includes this file and the
# definition of its kind in _units/.
#
# Two environment variables change where a run goes. tools/scripts/infra.sh
# sets them; nobody sets them by hand.
#
#   INFRA_OFFLINE=1    no backend and no account: `terraform validate` of
#                      every unit, with the outputs of other units mocked.
#   INFRA_EMULATOR=1   the state and the resources go to AWS Emulator on
#                      this machine instead of AWS: the rehearsal of an
#                      environment in `make check-all`.

locals {
  system = read_terragrunt_config("${get_parent_terragrunt_dir()}/system.hcl").locals
  env    = read_terragrunt_config(find_in_parent_folders("env.hcl")).locals

  offline  = get_env("INFRA_OFFLINE", "") != ""
  emulator = get_env("INFRA_EMULATOR", "") != ""

  # A rehearsal must never reach AWS: it stops here unless the endpoint of
  # every AWS service is one on this machine.
  emulator_endpoint = local.emulator ? regex("^https://(?:localhost|127\\.0\\.0\\.1):[0-9]+$", get_env("AWS_ENDPOINT_URL", "")) : ""

  # A bucket name is unique in the world, so it holds the account.
  state_bucket = "${local.system.system}-${local.env.name}-terraform-state-${local.env.account_id}"
}

# One state for each unit, in the bucket of its environment, locked with a
# lock file next to the state (`use_lockfile`). `make infra-bootstrap` creates
# the bucket once.
remote_state {
  backend = local.offline ? "local" : "s3"

  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }

  config = local.offline ? {} : merge(
    {
      bucket       = local.state_bucket
      key          = "${path_relative_to_include()}/terraform.tfstate"
      region       = local.env.region
      encrypt      = true
      use_lockfile = true
    },
    local.emulator ? {
      use_path_style              = true
      skip_credentials_validation = true
      skip_requesting_account_id  = true
    } : {},
  )
}

# The provider refuses every account but the one of the environment, so the
# credentials of stg cannot change prd by mistake, whatever the directory.
generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<-EOT
    provider "aws" {
      region = "${local.env.region}"
    %{if local.emulator~}

      # AWS Emulator: no account to check, and S3 by path.
      skip_credentials_validation = true
      skip_requesting_account_id  = true
      s3_use_path_style           = true
    %{else~}

      allowed_account_ids = ["${local.env.account_id}"]
    %{endif~}

      default_tags {
        tags = {
          Environment = "${local.env.name}"
          ManagedBy   = "terraform"
          Unit        = "${path_relative_to_include()}"
        }
      }
    }
  EOT
}

# Every unit installs the providers that providers.hcl pins, with the
# checksums of providers.lock.hcl, and may not change them:
# `-lockfile=readonly` fails on a provider that the lock file does not name
# (TPL-0024). `make infra-lock` updates the lock file after a change to
# providers.hcl.
generate "versions" {
  path              = "versions_override.tf"
  if_exists         = "overwrite"
  disable_signature = true
  contents          = file("${get_parent_terragrunt_dir()}/providers.hcl")
}

generate "lock" {
  path              = ".terraform.lock.hcl"
  if_exists         = "overwrite"
  disable_signature = true
  contents          = file("${get_parent_terragrunt_dir()}/providers.lock.hcl")
}

terraform {
  # The one lock file is providers.lock.hcl; no unit keeps a copy.
  copy_terraform_lock_file = false

  extra_arguments "locked_providers" {
    commands  = ["init"]
    arguments = ["-lockfile=readonly"]
  }
}
