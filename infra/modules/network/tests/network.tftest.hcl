# What the module plans, without an AWS account: the provider is a mock, so
# `terraform test` needs no credentials and creates nothing.

mock_provider "aws" {
  mock_data "aws_region" {
    defaults = {
      region = "ap-northeast-1"
    }
  }

  mock_data "aws_iam_policy_document" {
    defaults = {
      json = "{}"
    }
  }
}

variables {
  environment        = "test"
  availability_zones = ["ap-northeast-1a", "ap-northeast-1c"]
}

run "one_subnet_of_each_kind_in_each_zone" {
  command = plan

  assert {
    condition     = length(aws_subnet.public) == 2 && length(aws_subnet.private) == 2 && length(aws_subnet.database) == 2
    error_message = "Each zone needs one public, one private, and one database subnet."
  }

  assert {
    condition = length(distinct(concat(
      [for subnet in aws_subnet.public : subnet.cidr_block],
      [for subnet in aws_subnet.private : subnet.cidr_block],
      [for subnet in aws_subnet.database : subnet.cidr_block],
    ))) == 6
    error_message = "No two subnets may share an address range."
  }

  assert {
    condition     = alltrue([for subnet in aws_subnet.public : subnet.map_public_ip_on_launch == false])
    error_message = "A public subnet must not give a public address to what starts in it."
  }
}

run "no_route_to_the_internet_from_the_private_subnets_by_default" {
  command = plan

  assert {
    condition     = length(aws_nat_gateway.this) == 0 && length(aws_route.private_internet) == 0
    error_message = "By default, there is no NAT gateway, and no private subnet has a route to the internet."
  }
}

run "the_aws_services_are_reached_through_endpoints" {
  command = plan

  assert {
    condition     = aws_vpc_endpoint.s3.vpc_endpoint_type == "Gateway" && aws_vpc_endpoint.s3.service_name == "com.amazonaws.ap-northeast-1.s3"
    error_message = "Amazon S3 needs a gateway endpoint."
  }

  assert {
    condition = alltrue([
      for service in ["ecr.api", "ecr.dkr", "logs", "secretsmanager", "ecs-agent", "events", "sqs", "email"] :
      aws_vpc_endpoint.interface[service].service_name == "com.amazonaws.ap-northeast-1.${service}"
    ])
    error_message = "Each service that the tasks call needs an interface endpoint."
  }

  assert {
    condition     = alltrue([for endpoint in aws_vpc_endpoint.interface : endpoint.private_dns_enabled]) && length(local.endpoint_zones) == 2
    error_message = "An interface endpoint needs private DNS, and by default an interface in every zone."
  }

  assert {
    condition     = aws_vpc_security_group_ingress_rule.endpoints_https.cidr_ipv4 == "10.0.0.0/16" && aws_vpc_security_group_ingress_rule.endpoints_https.from_port == 443
    error_message = "The endpoints accept HTTPS from inside the VPC only."
  }
}

run "the_endpoints_can_stay_in_one_zone" {
  command = plan

  variables {
    interface_endpoints_in_every_zone = false
  }

  assert {
    condition     = length(local.endpoint_zones) == 1 && local.endpoint_zones[0] == "ap-northeast-1a"
    error_message = "With interface_endpoints_in_every_zone = false, an endpoint has one interface."
  }
}

run "one_nat_gateway_on_request" {
  command = plan

  variables {
    nat_gateways = "single"
  }

  assert {
    condition     = length(aws_nat_gateway.this) == 1 && length(aws_route.private_internet) == 2
    error_message = "With nat_gateways = single, the private subnets of every zone share one NAT gateway."
  }
}

run "one_nat_gateway_for_each_zone_on_request" {
  command = plan

  variables {
    nat_gateways = "per-zone"
  }

  assert {
    condition     = length(aws_nat_gateway.this) == 2 && length(aws_route.private_internet) == 2
    error_message = "With nat_gateways = per-zone, each zone needs a NAT gateway of its own."
  }
}

run "the_database_subnets_have_no_route_out" {
  command = plan

  assert {
    condition     = length(aws_route_table_association.database) == 2
    error_message = "Each database subnet needs the route table of the database subnets."
  }
}

run "one_zone_is_rejected" {
  command = plan

  variables {
    availability_zones = ["ap-northeast-1a"]
  }

  expect_failures = [var.availability_zones]
}

run "a_small_address_range_is_rejected" {
  command = plan

  variables {
    cidr_block = "10.0.0.0/24"
  }

  expect_failures = [var.cidr_block]
}

run "an_unknown_kind_of_nat_gateway_is_rejected" {
  command = plan

  variables {
    nat_gateways = "two"
  }

  expect_failures = [var.nat_gateways]
}
