# The web front end of an environment: infra/modules/web. The server of
# Next.js, behind the one public load balancer (TPL-0014).

locals {
  env      = read_terragrunt_config(find_in_parent_folders("env.hcl")).locals
  mocks    = read_terragrunt_config("${dirname(find_in_parent_folders("root.hcl"))}/mocks.hcl").locals
  emulator = get_env("INFRA_EMULATOR", "") != ""

  # The tag of the image: a hash of what the image is built from, so a
  # deployment restarts the server only when the frontend changed.
  image_tag = run_cmd("--terragrunt-quiet", "${get_repo_root()}/tools/scripts/images.sh", "tag", "web")
}

terraform {
  source = "${get_repo_root()}/infra/modules//web"
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

dependency "platform" {
  config_path = "../platform"

  mock_outputs                            = local.mocks.platform
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

# The APIs that the server calls. Their units are applied first.
dependency "facility" {
  config_path = "../contexts/facility"

  mock_outputs = {
    api_url = local.mocks.api_urls["facility"]
  }
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

dependency "reservation" {
  config_path = "../contexts/reservation"

  mock_outputs = {
    api_url = local.mocks.api_urls["reservation"]
  }
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

inputs = merge(
  {
    environment = local.env.name

    image          = "${dependency.registry.outputs.repository_urls["web"]}:${local.image_tag}"
    repository_arn = dependency.registry.outputs.repository_arns["web"]

    cluster_arn                = dependency.platform.outputs.cluster_arn
    namespace_arn              = dependency.platform.outputs.namespace_arn
    services_security_group_id = dependency.platform.outputs.services_security_group_id

    vpc_id            = dependency.network.outputs.vpc_id
    public_subnet_ids = dependency.network.outputs.public_subnet_ids
    subnet_ids        = dependency.network.outputs.private_subnet_ids

    domain_name         = local.env.domain_name
    hosted_zone_id      = local.env.hosted_zone_id
    desired_count       = local.env.desired_count
    deletion_protection = local.env.deletion_protection

    # The variables of the service `web` in compose.yaml.
    environment_variables = {
      FACILITY_API_URL    = dependency.facility.outputs.api_url
      RESERVATION_API_URL = dependency.reservation.outputs.api_url
    }
  },
  # AWS Emulator holds no hosted zone of the team and never issues a
  # certificate, so the rehearsal creates the one and requests the other
  # itself, and names them here (tools/scripts/infra.sh).
  local.emulator ? {
    hosted_zone_id  = get_env("INFRA_EMULATOR_HOSTED_ZONE_ID")
    certificate_arn = get_env("INFRA_EMULATOR_CERTIFICATE_ARN")
  } : {},
)
