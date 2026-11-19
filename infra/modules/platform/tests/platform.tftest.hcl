# What the module plans, without an AWS account: the provider is a mock, so
# `terraform test` needs no credentials and creates nothing.

mock_provider "aws" {}

variables {
  environment                 = "test"
  vpc_id                      = "vpc-0123456789abcdef0"
  endpoints_security_group_id = "sg-0endpoints"
  s3_prefix_list_id           = "pl-61a54008"
}

run "a_cluster_a_namespace_and_a_bus_for_the_environment" {
  command = plan

  assert {
    condition     = aws_ecs_cluster.this.name == "test" && aws_service_discovery_http_namespace.this.name == "test"
    error_message = "The cluster and the namespace are named after the environment."
  }

  assert {
    condition     = one([for setting in aws_ecs_cluster.this.setting : setting.value if setting.name == "containerInsights"]) == "enabled"
    error_message = "By default, Container Insights is on."
  }

  assert {
    condition     = module.event_bus.name == "contexts"
    error_message = "By default, the event bus has the name that the local environment uses."
  }
}

run "the_services_call_each_other_on_their_port" {
  command = plan

  assert {
    condition = (
      aws_vpc_security_group_ingress_rule.services_from_services.from_port == 8080
      && aws_vpc_security_group_ingress_rule.services_from_services.to_port == 8080
      && aws_vpc_security_group_ingress_rule.services_from_services.ip_protocol == "tcp"
    )
    error_message = "A task accepts TCP on 8080 from the services."
  }

  assert {
    condition = (
      aws_vpc_security_group_egress_rule.services_to_services.from_port == 8080
      && aws_vpc_security_group_egress_rule.services_to_services.to_port == 8080
    )
    error_message = "A task may connect to the services on 8080."
  }
}

run "the_services_reach_aws_through_the_endpoints" {
  command = plan

  assert {
    condition = (
      aws_vpc_security_group_egress_rule.services_to_endpoints.referenced_security_group_id == "sg-0endpoints"
      && aws_vpc_security_group_egress_rule.services_to_endpoints.from_port == 443
      && aws_vpc_security_group_egress_rule.services_to_endpoints.to_port == 443
    )
    error_message = "A task may connect to the interface endpoints on 443."
  }

  assert {
    condition = (
      aws_vpc_security_group_egress_rule.services_to_s3.prefix_list_id == "pl-61a54008"
      && aws_vpc_security_group_egress_rule.services_to_s3.from_port == 443
      && aws_vpc_security_group_egress_rule.services_to_s3.to_port == 443
    )
    error_message = "A task may connect to Amazon S3 on 443, through the prefix list of its gateway endpoint."
  }
}

run "no_rule_names_an_address_range" {
  command = plan

  assert {
    condition = alltrue([
      for rule in [
        aws_vpc_security_group_ingress_rule.services_from_services,
        aws_vpc_security_group_egress_rule.services_to_services,
        aws_vpc_security_group_egress_rule.services_to_endpoints,
        aws_vpc_security_group_egress_rule.services_to_s3,
      ] : rule.cidr_ipv4 == null && rule.cidr_ipv6 == null
    ])
    error_message = "A rule of the services names a security group or a prefix list, never an address range."
  }
}

run "container_insights_can_be_turned_off" {
  command = plan

  variables {
    container_insights = "disabled"
  }

  assert {
    condition     = one([for setting in aws_ecs_cluster.this.setting : setting.value if setting.name == "containerInsights"]) == "disabled"
    error_message = "With container_insights = disabled, Container Insights is off."
  }
}

run "an_unknown_setting_of_container_insights_is_rejected" {
  command = plan

  variables {
    container_insights = "on"
  }

  expect_failures = [var.container_insights]
}
