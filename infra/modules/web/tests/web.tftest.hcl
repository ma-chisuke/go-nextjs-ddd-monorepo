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
  environment                = "test"
  image                      = "111111111111.dkr.ecr.ap-northeast-1.amazonaws.com/web:0123456789abcdef"
  repository_arn             = "arn:aws:ecr:ap-northeast-1:111111111111:repository/web"
  cluster_arn                = "arn:aws:ecs:ap-northeast-1:111111111111:cluster/test"
  namespace_arn              = "arn:aws:servicediscovery:ap-northeast-1:111111111111:namespace/ns-0000000000000000"
  vpc_id                     = "vpc-00000000000000000"
  public_subnet_ids          = ["subnet-00000000000000001", "subnet-00000000000000002"]
  subnet_ids                 = ["subnet-00000000000000003", "subnet-00000000000000004"]
  services_security_group_id = "sg-00000000000000001"
  domain_name                = "test.example.com"
  hosted_zone_id             = "Z00000000000000000000"

  environment_variables = {
    RESERVATION_API_URL = "http://reservation:8080"
    FACILITY_API_URL    = "http://facility:8080"
  }
}

run "the_load_balancer_serves_https_and_redirects_http" {
  command = plan

  assert {
    condition     = aws_lb.this.internal == false && aws_lb.this.drop_invalid_header_fields == true
    error_message = "The load balancer is the public entry, and drops invalid header fields."
  }

  assert {
    condition     = aws_lb_listener.https.protocol == "HTTPS" && aws_lb_listener.https.ssl_policy == "ELBSecurityPolicy-TLS13-1-2-2021-06"
    error_message = "The site is served over HTTPS, with TLS 1.2 or later."
  }

  assert {
    condition     = aws_lb_listener.http.default_action[0].type == "redirect" && aws_lb_listener.http.default_action[0].redirect[0].protocol == "HTTPS"
    error_message = "Port 80 only redirects to HTTPS."
  }

  assert {
    condition     = aws_lb_target_group.this.health_check[0].path == "/healthz"
    error_message = "The load balancer checks the health endpoint, which calls no API."
  }
}

run "the_tasks_are_private" {
  command = plan

  assert {
    condition     = aws_ecs_service.this.network_configuration[0].assign_public_ip == false
    error_message = "A task has no public address."
  }

  assert {
    condition     = aws_ecs_service.this.network_configuration[0].subnets == toset(["subnet-00000000000000003", "subnet-00000000000000004"])
    error_message = "The tasks run in the private subnets."
  }

  assert {
    condition     = aws_vpc_security_group_ingress_rule.from_load_balancer.cidr_ipv4 == null && aws_vpc_security_group_ingress_rule.from_load_balancer.from_port == 3000
    error_message = "The tasks accept requests from the load balancer alone."
  }

  assert {
    condition     = aws_vpc_security_group_egress_rule.to_tasks.cidr_ipv4 == null
    error_message = "The load balancer sends to the tasks alone."
  }
}

run "the_server_gets_its_settings_and_can_be_rolled_back" {
  command = plan

  assert {
    condition = jsondecode(aws_ecs_task_definition.this.container_definitions)[0].environment == [
      { name = "FACILITY_API_URL", value = "http://facility:8080" },
      { name = "RESERVATION_API_URL", value = "http://reservation:8080" },
    ]
    error_message = "The server gets the environment variables, in the order of their names."
  }

  assert {
    condition     = jsondecode(aws_ecs_task_definition.this.container_definitions)[0].readonlyRootFilesystem == true
    error_message = "The root file system of the container is read-only."
  }

  assert {
    condition     = aws_ecs_service.this.deployment_circuit_breaker[0].rollback == true
    error_message = "A deployment that fails is rolled back."
  }

  assert {
    condition     = aws_ecs_service.this.service_connect_configuration[0].enabled == true && length(aws_ecs_service.this.service_connect_configuration[0].service) == 0
    error_message = "The service is a client of Service Connect and offers no name."
  }
}

run "no_task_is_rejected" {
  command = plan

  variables {
    desired_count = 0
  }

  expect_failures = [var.desired_count]
}

run "an_unknown_architecture_is_rejected" {
  command = plan

  variables {
    cpu_architecture = "arm"
  }

  expect_failures = [var.cpu_architecture]
}

run "a_certificate_is_requested_by_default" {
  command = plan

  assert {
    condition     = length(aws_acm_certificate.this) == 1 && aws_acm_certificate.this[0].domain_name == "test.example.com" && length(aws_acm_certificate_validation.this) == 1
    error_message = "Without certificate_arn, the module requests a certificate for the domain name and waits until it is issued."
  }
}

run "a_given_certificate_is_used" {
  command = plan

  variables {
    certificate_arn = "arn:aws:acm:ap-northeast-1:111111111111:certificate/00000000-0000-0000-0000-000000000000"
  }

  assert {
    condition     = length(aws_acm_certificate.this) == 0 && length(aws_route53_record.validation) == 0
    error_message = "With certificate_arn, the module requests no certificate."
  }

  assert {
    condition     = aws_lb_listener.https.certificate_arn == "arn:aws:acm:ap-northeast-1:111111111111:certificate/00000000-0000-0000-0000-000000000000"
    error_message = "The listener serves the certificate that was given."
  }
}
