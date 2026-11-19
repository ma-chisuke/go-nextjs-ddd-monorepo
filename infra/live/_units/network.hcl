# The network of an environment: infra/modules/network.

locals {
  env      = read_terragrunt_config(find_in_parent_folders("env.hcl")).locals
  emulator = get_env("INFRA_EMULATOR", "") != ""
}

terraform {
  source = "${get_repo_root()}/infra/modules//network"
}

inputs = merge(
  {
    environment                       = local.env.name
    availability_zones                = local.env.availability_zones
    interface_endpoints_in_every_zone = local.env.interface_endpoints_in_every_zone
  },
  # AWS Emulator does not know the endpoint of the API of Amazon SES, so the
  # rehearsal leaves it out of the default list of the module.
  local.emulator ? {
    interface_endpoints = ["ecr.api", "ecr.dkr", "logs", "secretsmanager", "ecs-agent", "events", "sqs"]
  } : {},
)
