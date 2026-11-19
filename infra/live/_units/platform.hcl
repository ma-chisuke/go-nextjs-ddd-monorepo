# What every service of an environment shares: infra/modules/platform.

locals {
  system = read_terragrunt_config("${dirname(find_in_parent_folders("root.hcl"))}/system.hcl").locals
  env    = read_terragrunt_config(find_in_parent_folders("env.hcl")).locals
  mocks  = read_terragrunt_config("${dirname(find_in_parent_folders("root.hcl"))}/mocks.hcl").locals
}

terraform {
  source = "${get_repo_root()}/infra/modules//platform"
}

dependency "network" {
  config_path = "../network"

  mock_outputs                            = local.mocks.network
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

inputs = {
  environment                 = local.env.name
  vpc_id                      = dependency.network.outputs.vpc_id
  endpoints_security_group_id = dependency.network.outputs.endpoints_security_group_id
  s3_prefix_list_id           = dependency.network.outputs.s3_prefix_list_id
  event_bus_name              = local.system.event_bus_name
}
