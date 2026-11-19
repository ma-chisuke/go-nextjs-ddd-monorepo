# The Notification context: infra/modules/context-service. One process, a
# worker that consumes the events of Reservation from its queue and sends
# mail through Amazon SES (TPL-0013). It has no API.

locals {
  env   = read_terragrunt_config(find_in_parent_folders("env.hcl")).locals
  mocks = read_terragrunt_config("${dirname(find_in_parent_folders("root.hcl"))}/mocks.hcl").locals

  name = "notification"

  # The tag of the image: a hash of what the image is built from, so a
  # deployment restarts this context only when it changed.
  image_tag = run_cmd("--terragrunt-quiet", "${get_repo_root()}/tools/scripts/images.sh", "tag", local.name)
}

terraform {
  source = "${get_repo_root()}/infra/modules//context-service"
}

dependency "network" {
  config_path = "../../network"

  mock_outputs                            = local.mocks.network
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

dependency "registry" {
  config_path = "../../registry"

  mock_outputs                            = local.mocks.registry
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

dependency "database" {
  config_path = "../../database"

  mock_outputs                            = local.mocks.database
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

dependency "platform" {
  config_path = "../../platform"

  mock_outputs                            = local.mocks.platform
  mock_outputs_allowed_terraform_commands = ["init", "validate", "plan"]
}

inputs = {
  environment = local.env.name
  name        = local.name

  image          = "${dependency.registry.outputs.repository_urls[local.name]}:${local.image_tag}"
  repository_arn = dependency.registry.outputs.repository_arns[local.name]

  cluster_arn                = dependency.platform.outputs.cluster_arn
  namespace_arn              = dependency.platform.outputs.namespace_arn
  services_security_group_id = dependency.platform.outputs.services_security_group_id
  event_bus                  = dependency.platform.outputs.event_bus

  vpc_id     = dependency.network.outputs.vpc_id
  subnet_ids = dependency.network.outputs.private_subnet_ids

  database = {
    security_group_id = dependency.database.outputs.security_group_id
    secret_arn        = dependency.database.outputs.runtime_secret_arns[local.name]
  }

  # The command of the image is the worker.
  workers = {
    worker = {
      command       = []
      desired_count = local.env.desired_count
    }
  }

  # The events of contracts/events/notification.asyncapi.yaml, as in the
  # local system (infra/local/main.tf).
  subscription = {
    sources = ["/contexts/reservation"]
    event_types = [
      "reservation.ReservationConfirmed.v1",
      "reservation.ReservationCancelled.v1",
    ]
  }

  # Without SMTP_ADDR the worker sends through Amazon SES, from an address
  # that the verified identity of the environment covers.
  environment_variables = {
    MAIL_FROM = local.env.mail_from
  }

  iam_statements = [
    {
      actions   = ["ses:SendEmail"]
      resources = ["arn:aws:ses:${local.env.region}:${local.env.account_id}:identity/${local.env.mail_identity}"]
    },
  ]
}
