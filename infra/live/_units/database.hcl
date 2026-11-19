# The database of an environment: infra/modules/database. One Aurora cluster
# for every context, the secrets of the roles of each context, and the task
# that prepares the database before a deployment (TPL-0009, TPL-0016).

locals {
  system   = read_terragrunt_config("${dirname(find_in_parent_folders("root.hcl"))}/system.hcl").locals
  env      = read_terragrunt_config(find_in_parent_folders("env.hcl")).locals
  mocks    = read_terragrunt_config("${dirname(find_in_parent_folders("root.hcl"))}/mocks.hcl").locals
  emulator = get_env("INFRA_EMULATOR", "") != ""

  # The tag of the image that prepares the database: a hash of db/ and of
  # the schema of every context.
  image_tag = run_cmd("--terragrunt-quiet", "${get_repo_root()}/tools/scripts/images.sh", "tag", "db")
}

terraform {
  source = "${get_repo_root()}/infra/modules//database"
}

dependency "network" {
  config_path = "../network"

  mock_outputs                            = local.mocks.network
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

dependency "registry" {
  config_path = "../registry"

  mock_outputs                            = local.mocks.registry
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

inputs = merge(
  {
    environment = local.env.name
    contexts    = local.system.contexts

    vpc_id                      = dependency.network.outputs.vpc_id
    subnet_ids                  = dependency.network.outputs.database_subnet_ids
    task_subnet_ids             = dependency.network.outputs.private_subnet_ids
    endpoints_security_group_id = dependency.network.outputs.endpoints_security_group_id
    s3_prefix_list_id           = dependency.network.outputs.s3_prefix_list_id

    capacity            = local.env.database_capacity
    instance_count      = local.env.database_instance_count
    deletion_protection = local.env.deletion_protection

    db_image = "${dependency.registry.outputs.repository_urls["db"]}:${local.image_tag}"
  },
  # AWS Emulator knows Aurora PostgreSQL up to version 17. It names a
  # repository localhost:<port>/<name>, and the module accepts only an
  # address of Amazon ECR, so the rehearsal writes the address as AWS would.
  local.emulator ? {
    engine_version = "17.9"
    db_image       = "000000000000.dkr.ecr.${local.env.region}.amazonaws.com/db:${local.image_tag}"
  } : {},
)
