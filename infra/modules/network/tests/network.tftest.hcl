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

run "one_nat_gateway_by_default" {
  command = plan

  assert {
    condition     = length(aws_nat_gateway.this) == 1
    error_message = "By default, the zones share one NAT gateway."
  }

  assert {
    condition     = length(aws_route.private_internet) == 2
    error_message = "The private subnets of every zone need a route to the NAT gateway."
  }
}

run "one_nat_gateway_for_each_zone_on_request" {
  command = plan

  variables {
    single_nat_gateway = false
  }

  assert {
    condition     = length(aws_nat_gateway.this) == 2
    error_message = "With single_nat_gateway = false, each zone needs a NAT gateway of its own."
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
