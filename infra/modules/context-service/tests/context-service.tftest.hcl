# What the module plans, without an AWS account: the provider is a mock, so
# `terraform test` needs no credentials and creates nothing. The mock makes
# up the values that AWS would compute, such as the URL of a queue, during
# the plan already, so that the assertions can read them.

mock_provider "aws" {
  override_during = plan

  mock_data "aws_region" {
    defaults = {
      region = "ap-northeast-1"
    }
  }

  mock_data "aws_caller_identity" {
    defaults = {
      account_id = "123456789012"
    }
  }

  mock_data "aws_partition" {
    defaults = {
      partition = "aws"
    }
  }

  mock_data "aws_iam_policy_document" {
    defaults = {
      json = "{}"
    }
  }

  mock_resource "aws_cloudwatch_log_group" {
    defaults = {
      arn = "arn:aws:logs:ap-northeast-1:123456789012:log-group:/test/reservation"
    }
  }

  mock_resource "aws_security_group" {
    defaults = {
      id = "sg-0context"
    }
  }

  mock_resource "aws_sqs_queue" {
    defaults = {
      url = "https://sqs.ap-northeast-1.amazonaws.com/123456789012/notification-events"
      arn = "arn:aws:sqs:ap-northeast-1:123456789012:notification-events"
    }
  }
}

# Reservation: an API, and the relay of its outbox, which publishes events.
variables {
  environment    = "test"
  name           = "reservation"
  image          = "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/reservation:1.0.0"
  repository_arn = "arn:aws:ecr:ap-northeast-1:123456789012:repository/reservation"
  cluster_arn    = "arn:aws:ecs:ap-northeast-1:123456789012:cluster/test"
  namespace_arn  = "arn:aws:servicediscovery:ap-northeast-1:123456789012:namespace/ns-test"
  vpc_id         = "vpc-0test"
  subnet_ids     = ["subnet-0a", "subnet-0c"]

  services_security_group_id = "sg-0services"

  database = {
    security_group_id = "sg-0database"
    secret_arn        = "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:test/reservation-AbCdEf"
  }

  api = {}
  workers = {
    relay = { command = ["relay"] }
  }

  environment_variables = {
    FACILITY_API_URL = "http://facility:8080"
  }

  event_bus = {
    name = "contexts"
    arn  = "arn:aws:events:ap-northeast-1:123456789012:event-bus/contexts"
  }
  publishes_events = true
}

run "an_api_and_a_worker_that_publishes" {
  command = plan

  assert {
    condition     = toset(keys(aws_ecs_service.this)) == toset(["reservation", "reservation-relay"])
    error_message = "The API is the service <name>, and each worker the service <name>-<key>."
  }

  assert {
    condition     = output.service_names == tolist(["reservation", "reservation-relay"])
    error_message = "service_names lists the services of the context."
  }

  assert {
    condition     = !contains(keys(jsondecode(aws_ecs_task_definition.this["reservation"].container_definitions)[0]), "command")
    error_message = "The API runs the image with no command."
  }

  assert {
    condition     = jsondecode(aws_ecs_task_definition.this["reservation-relay"].container_definitions)[0].command == ["relay"]
    error_message = "The relay runs the image with the argument relay."
  }

  assert {
    condition = alltrue([
      for definition in aws_ecs_task_definition.this :
      jsondecode(definition.container_definitions)[0].image == "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/reservation:1.0.0"
    ])
    error_message = "Every process runs the image of the context."
  }

  assert {
    condition = alltrue([
      for definition in aws_ecs_task_definition.this : {
        for variable in jsondecode(definition.container_definitions)[0].environment : variable.name => variable.value
        } == {
        AWS_REGION       = "ap-northeast-1"
        EVENT_BUS_NAME   = "contexts"
        FACILITY_API_URL = "http://facility:8080"
      }
    ])
    error_message = "Every process gets the variables of the context, the Region, and the name of the bus."
  }

  assert {
    condition     = output.api_url == "http://reservation:8080" && output.events_queue_url == null
    error_message = "The API is reached at http://<name>:8080, and a context without a subscription has no queue."
  }
}

run "the_api_serves_through_service_connect" {
  command = plan

  assert {
    condition = (
      aws_ecs_service.this["reservation"].service_connect_configuration[0].enabled
      && aws_ecs_service.this["reservation"].service_connect_configuration[0].namespace == "arn:aws:servicediscovery:ap-northeast-1:123456789012:namespace/ns-test"
      && length(aws_ecs_service.this["reservation"].service_connect_configuration[0].service) == 1
    )
    error_message = "The API is a service of Service Connect in the namespace of the environment."
  }

  assert {
    condition = (
      aws_ecs_service.this["reservation"].service_connect_configuration[0].service[0].port_name == "http"
      && aws_ecs_service.this["reservation"].service_connect_configuration[0].service[0].discovery_name == "reservation"
      && aws_ecs_service.this["reservation"].service_connect_configuration[0].service[0].client_alias[0].dns_name == "reservation"
      && aws_ecs_service.this["reservation"].service_connect_configuration[0].service[0].client_alias[0].port == 8080
    )
    error_message = "The API serves under the name of the context, on 8080."
  }

  assert {
    condition = jsondecode(aws_ecs_task_definition.this["reservation"].container_definitions)[0].portMappings == [{
      name          = "http"
      containerPort = 8080
      protocol      = "tcp"
      appProtocol   = "http"
    }]
    error_message = "The container of the API names its port http, the name by which Service Connect knows it."
  }

  assert {
    condition = (
      aws_ecs_service.this["reservation-relay"].service_connect_configuration[0].enabled
      && length(aws_ecs_service.this["reservation-relay"].service_connect_configuration[0].service) == 0
      && !contains(keys(jsondecode(aws_ecs_task_definition.this["reservation-relay"].container_definitions)[0]), "portMappings")
    )
    error_message = "A worker is a client of Service Connect only, and publishes no port."
  }
}

run "nothing_can_be_reached_from_outside" {
  command = plan

  assert {
    condition = alltrue([
      for service in aws_ecs_service.this :
      service.network_configuration[0].assign_public_ip == false
      && toset(service.network_configuration[0].subnets) == toset(["subnet-0a", "subnet-0c"])
      && length(service.load_balancer) == 0
    ])
    error_message = "A service runs in the private subnets, with no public address and no load balancer."
  }

  assert {
    condition = alltrue([
      for rule in [aws_vpc_security_group_egress_rule.database, aws_vpc_security_group_ingress_rule.database] :
      rule.cidr_ipv4 == null && rule.cidr_ipv6 == null && rule.prefix_list_id == null
    ])
    error_message = "A rule of the context names a security group, never an address range."
  }

  assert {
    condition = (
      aws_vpc_security_group_egress_rule.database.referenced_security_group_id == "sg-0database"
      && aws_vpc_security_group_egress_rule.database.from_port == 5432
      && aws_vpc_security_group_ingress_rule.database.security_group_id == "sg-0database"
      && aws_vpc_security_group_ingress_rule.database.from_port == 5432
    )
    error_message = "The tasks reach PostgreSQL, and the database accepts them."
  }

  assert {
    condition = alltrue([
      for service in aws_ecs_service.this :
      service.network_configuration[0].security_groups == toset(["sg-0context", "sg-0services"])
    ])
    error_message = "A task joins the group of its context and the group of the services."
  }
}

run "the_tasks_are_hardened" {
  command = plan

  assert {
    condition = alltrue([
      for definition in aws_ecs_task_definition.this :
      definition.network_mode == "awsvpc" && definition.requires_compatibilities == toset(["FARGATE"])
    ])
    error_message = "A task runs on Fargate in the network mode awsvpc."
  }

  assert {
    condition = alltrue([
      for definition in aws_ecs_task_definition.this :
      jsondecode(definition.container_definitions)[0].readonlyRootFilesystem == true
      && jsondecode(definition.container_definitions)[0].linuxParameters.capabilities == { drop = ["ALL"] }
      && !contains(keys(jsondecode(definition.container_definitions)[0]), "privileged")
    ])
    error_message = "A container has a read-only root file system and no Linux capability."
  }

  assert {
    condition = alltrue([
      for definition in aws_ecs_task_definition.this :
      jsondecode(definition.container_definitions)[0].healthCheck.command == ["CMD", "/service", "healthcheck"]
    ])
    error_message = "Every process, a worker too, is checked with the health check of the image."
  }

  assert {
    condition = alltrue([
      for definition in aws_ecs_task_definition.this :
      jsondecode(definition.container_definitions)[0].logConfiguration.options["awslogs-group"] == "/test/reservation"
    ])
    error_message = "Every process writes its logs to the log group of the context."
  }

  assert {
    condition = alltrue([
      for service in aws_ecs_service.this :
      service.enable_execute_command == false
      && service.deployment_circuit_breaker[0].enable && service.deployment_circuit_breaker[0].rollback
      && service.propagate_tags == "SERVICE"
    ])
    error_message = "A service has no ECS Exec, rolls a failed deployment back, and tags its tasks."
  }
}

run "the_database_url_comes_from_the_secret" {
  command = plan

  assert {
    condition = alltrue([
      for definition in aws_ecs_task_definition.this :
      jsondecode(definition.container_definitions)[0].secrets == [{
        name      = "DATABASE_URL"
        valueFrom = "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:test/reservation-AbCdEf:url::"
      }]
      && !contains([for variable in jsondecode(definition.container_definitions)[0].environment : variable.name], "DATABASE_URL")
    ])
    error_message = "DATABASE_URL comes from the key url of the secret, and never as a plain variable."
  }
}

run "the_execution_role_starts_the_tasks_of_this_context_alone" {
  command = plan

  assert {
    condition = {
      for statement in local.execution_statements : statement.sid => toset(statement.resources)
      } == {
      SignInToTheRegistry = toset(["*"])
      PullTheImage        = toset(["arn:aws:ecr:ap-northeast-1:123456789012:repository/reservation"])
      WriteTheLogs        = toset(["arn:aws:logs:ap-northeast-1:123456789012:log-group:/test/reservation:*"])
      ReadTheSecrets      = toset(["arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:test/reservation-AbCdEf"])
    }
    error_message = "The execution role may pull the image of the context, write its logs, and read its secret; only the token of the registry takes no resource."
  }

  assert {
    condition     = length(data.aws_iam_policy_document.execution.statement) == length(local.execution_statements)
    error_message = "The policy of the execution role holds the statements of local.execution_statements."
  }
}

run "a_secret_under_a_key_of_the_team_needs_that_key" {
  command = plan

  variables {
    database = {
      security_group_id = "sg-0database"
      secret_arn        = "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:test/reservation-AbCdEf"
      kms_key_arn       = "arn:aws:kms:ap-northeast-1:123456789012:key/0123abcd-0123-abcd-0123-0123456789ab"
    }
  }

  assert {
    condition = toset(one([
      for statement in local.execution_statements : statement.resources
      if statement.sid == "DecryptTheSecrets"
    ])) == toset(["arn:aws:kms:ap-northeast-1:123456789012:key/0123abcd-0123-abcd-0123-0123456789ab"])
    error_message = "With database.kms_key_arn, the execution role may decrypt with that key alone."
  }
}

run "the_role_of_a_publisher_may_put_events_on_the_bus" {
  command = plan

  assert {
    condition     = [for statement in local.task_statements : statement.sid] == ["PublishEvents"]
    error_message = "A context that publishes and consumes nothing may use the bus alone."
  }

  assert {
    condition = (
      toset(local.task_statements[0].actions) == toset(["events:DescribeEventBus", "events:PutEvents"])
      && toset(local.task_statements[0].resources) == toset(["arn:aws:events:ap-northeast-1:123456789012:event-bus/contexts"])
    )
    error_message = "The relay may describe the bus and put events on it, on that bus alone."
  }

  assert {
    condition     = length(aws_iam_role_policy.task) == 1
    error_message = "The role of the tasks needs its policy."
  }
}

# Notification: a worker alone, which runs the image as it is and reads the
# queue of the context.
run "a_worker_alone_that_consumes" {
  command = plan

  variables {
    name           = "notification"
    image          = "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/notification:1.0.0"
    repository_arn = "arn:aws:ecr:ap-northeast-1:123456789012:repository/notification"
    api            = null
    workers = {
      worker = { command = [] }
    }
    environment_variables = {
      SMTP_ADDR = "email-smtp.ap-northeast-1.amazonaws.com:587"
    }
    publishes_events = false
    subscription = {
      sources     = ["/contexts/reservation"]
      event_types = ["reservation.ReservationConfirmed.v1", "reservation.ReservationCancelled.v1"]
    }
    iam_statements = [{
      actions   = ["ses:SendEmail"]
      resources = ["arn:aws:ses:ap-northeast-1:123456789012:identity/example.com"]
    }]
  }

  assert {
    condition     = keys(aws_ecs_service.this) == ["notification-worker"]
    error_message = "A context without an API runs its worker alone."
  }

  assert {
    condition     = !contains(keys(jsondecode(aws_ecs_task_definition.this["notification-worker"].container_definitions)[0]), "command")
    error_message = "A worker with an empty command runs the image as it is."
  }

  assert {
    condition = {
      for variable in jsondecode(aws_ecs_task_definition.this["notification-worker"].container_definitions)[0].environment : variable.name => variable.value
      } == {
      AWS_REGION       = "ap-northeast-1"
      EVENTS_QUEUE_URL = "https://sqs.ap-northeast-1.amazonaws.com/123456789012/notification-events"
      SMTP_ADDR        = "email-smtp.ap-northeast-1.amazonaws.com:587"
    }
    error_message = "The worker gets the URL of its queue, and no name of a bus."
  }

  assert {
    condition     = output.api_url == null && output.events_queue_url == "https://sqs.ap-northeast-1.amazonaws.com/123456789012/notification-events"
    error_message = "A context without an API has no URL, and a subscriber has a queue."
  }

  assert {
    condition = (
      module.subscription[0].queue_url == output.events_queue_url
      && length(aws_ecs_service.this["notification-worker"].service_connect_configuration[0].service) == 0
    )
    error_message = "The subscription is the queue of the context, and the worker serves nothing."
  }

  assert {
    condition     = [for statement in local.task_statements : statement.sid] == ["ConsumeEvents", null]
    error_message = "A context that consumes and does not publish may use its queue, and what iam_statements add; not the bus."
  }

  assert {
    condition = (
      toset(local.task_statements[0].actions) == toset(["sqs:DeleteMessage", "sqs:ReceiveMessage"])
      && toset(local.task_statements[0].resources) == toset(["arn:aws:sqs:ap-northeast-1:123456789012:notification-events"])
      && toset(local.task_statements[1].actions) == toset(["ses:SendEmail"])
    )
    error_message = "The worker may receive and delete the messages of its own queue alone."
  }
}

# Facility: an API alone, which neither publishes nor consumes.
run "an_api_alone_calls_no_aws_service" {
  command = plan

  variables {
    name                  = "facility"
    workers               = {}
    publishes_events      = false
    environment_variables = {}
  }

  assert {
    condition     = keys(aws_ecs_service.this) == ["facility"]
    error_message = "A context with an API alone runs one service."
  }

  assert {
    condition     = length(local.task_statements) == 0 && length(aws_iam_role_policy.task) == 0
    error_message = "The role of a context that calls no AWS service has no permission."
  }

  assert {
    condition     = length(module.subscription) == 0
    error_message = "A context without a subscription has no queue."
  }
}

run "a_plain_database_url_is_rejected" {
  command = plan

  variables {
    environment_variables = {
      DATABASE_URL = "postgres://reservation_runtime:secret@db:5432/app"
    }
  }

  expect_failures = [var.environment_variables]
}

run "a_context_that_runs_nothing_is_rejected" {
  command = plan

  variables {
    api     = null
    workers = {}
  }

  expect_failures = [var.workers]
}

run "a_size_that_fargate_does_not_run_is_rejected" {
  command = plan

  variables {
    api = { cpu = 256, memory = 4096 }
  }

  expect_failures = [var.api]
}

run "an_image_without_a_tag_is_rejected" {
  command = plan

  variables {
    image = "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/reservation"
  }

  expect_failures = [var.image]
}
