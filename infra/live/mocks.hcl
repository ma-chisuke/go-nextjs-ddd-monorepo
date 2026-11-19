# What a unit sees of a unit that it depends on while that unit has no state
# yet: in `terraform validate` without an account, and in a plan before the
# first apply. An apply never uses these values (`mock_outputs_allowed_
# terraform_commands` in _units/). The values only have the shape of the real
# outputs.

locals {
  system = read_terragrunt_config("${get_terragrunt_dir()}/system.hcl").locals

  images = concat(local.system.contexts, ["web", "db"])

  network = {
    vpc_id                      = "vpc-00000000000000000"
    availability_zones          = ["ap-northeast-1a", "ap-northeast-1c"]
    public_subnet_ids           = ["subnet-00000000000000001", "subnet-00000000000000002"]
    private_subnet_ids          = ["subnet-00000000000000003", "subnet-00000000000000004"]
    database_subnet_ids         = ["subnet-00000000000000005", "subnet-00000000000000006"]
    endpoints_security_group_id = "sg-00000000000000001"
    s3_prefix_list_id           = "pl-00000000"
  }

  registry = {
    repository_urls = { for image in local.images : image => "000000000000.dkr.ecr.ap-northeast-1.amazonaws.com/${image}" }
    repository_arns = { for image in local.images : image => "arn:aws:ecr:ap-northeast-1:000000000000:repository/${image}" }
  }

  database = {
    security_group_id = "sg-00000000000000002"
    runtime_secret_arns = {
      for context in local.system.contexts :
      context => "arn:aws:secretsmanager:ap-northeast-1:000000000000:secret:mock/database/${context}/runtime-000000"
    }
  }

  platform = {
    cluster_arn                = "arn:aws:ecs:ap-northeast-1:000000000000:cluster/mock"
    namespace_arn              = "arn:aws:servicediscovery:ap-northeast-1:000000000000:namespace/ns-0000000000000000"
    services_security_group_id = "sg-00000000000000003"
    event_bus = {
      name = local.system.event_bus_name
      arn  = "arn:aws:events:ap-northeast-1:000000000000:event-bus/${local.system.event_bus_name}"
    }
  }

  # A context with an API answers at this address (infra/modules/context-service).
  api_urls = { for context in local.system.contexts : context => "http://${context}:8080" }
}
