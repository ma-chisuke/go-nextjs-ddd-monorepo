# Everything of one bounded context on ECS (TPL-0014): the service of its API,
# if it has one, a service for each worker, the roles of its tasks, its logs,
# its security group, and its subscription to events (TPL-0013).
#
# The processes are those of compose.yaml, from the same image and with the
# same commands. The API runs the image as it is; a worker runs it with the
# name of its mode as its argument, such as `relay`, or as it is when the
# context has no API, as Notification does.
#
# Nothing here can be reached from outside the VPC: no task gets a public
# address, no service is behind a load balancer, and no variable could add
# either. Other contexts call the API through ECS Service Connect, at
# http://<name>:8080, the address that it has in Docker Compose too. Only the
# web front end has a public entry, in a module of its own.

locals {
  # The port on which every process listens: HTTP_ADDR of the services, by
  # default (backend/internal/platform/httpserver). The API serves its
  # requests there, and every process its health check.
  port = 8080

  # The processes, by the name of their ECS service. The API is the service
  # <name>; each worker is the service <name>-<key>.
  api = var.api == null ? {} : {
    (var.name) = {
      log_prefix    = "api"
      command       = null
      desired_count = var.api.desired_count
      cpu           = var.api.cpu
      memory        = var.api.memory
      serves        = true
    }
  }
  workers = {
    for key, worker in var.workers : "${var.name}-${key}" => {
      log_prefix    = key
      command       = length(worker.command) > 0 ? worker.command : null
      desired_count = worker.desired_count
      cpu           = worker.cpu
      memory        = worker.memory
      serves        = false
    }
  }
  processes = merge(local.api, local.workers)

  # Every process of the context gets the same configuration, as in
  # compose.yaml (platform/config). A variable that a process does not read
  # does no harm.
  environment = merge(
    var.environment_variables,
    { AWS_REGION = data.aws_region.current.region },
    var.publishes_events ? { EVENT_BUS_NAME = var.event_bus.name } : {},
    var.subscription == null ? {} : { EVENTS_QUEUE_URL = module.subscription[0].queue_url },
  )

  # The secrets come from Secrets Manager when a task starts, so the code
  # calls no AWS API to read them (TPL-0014), and the task definition holds
  # their ARNs, not their values. The secret of the database is JSON; ECS
  # takes the key `url` from it.
  secrets = {
    DATABASE_URL = "${var.database.secret_arn}:url::"
  }

  # The container of each process. A key whose value is null is left out, so
  # that the definition says only what it sets.
  containers = {
    for service, process in local.processes : service => {
      for key, value in {
        name      = var.name
        image     = var.image
        essential = true
        command   = process.command

        # The API gives its port a name, by which Service Connect knows it. A
        # worker publishes no port: its health check stays inside the task.
        portMappings = process.serves ? [{
          name          = "http"
          containerPort = local.port
          protocol      = "tcp"
          appProtocol   = "http"
        }] : null

        environment = [for name, value in local.environment : { name = name, value = value }]
        secrets     = [for name, value_from in local.secrets : { name = name, valueFrom = value_from }]

        # The command of the HEALTHCHECK of the image (backend/Dockerfile),
        # which ECS does not read, with the same timing. A worker answers it
        # too.
        healthCheck = {
          command     = ["CMD", "/service", "healthcheck"]
          interval    = 10
          timeout     = 3
          retries     = 3
          startPeriod = 30
        }

        # As in compose.yaml: the process writes no file, and needs no
        # Linux capability. The image runs it as a user other than root.
        readonlyRootFilesystem = true
        linuxParameters = {
          capabilities = {
            drop = ["ALL"]
          }
        }

        logConfiguration = {
          logDriver = "awslogs"
          options = {
            awslogs-group         = aws_cloudwatch_log_group.this.name
            awslogs-region        = data.aws_region.current.region
            awslogs-stream-prefix = process.log_prefix
          }
        }
      } : key => value if value != null
    }
  }
}

data "aws_caller_identity" "current" {}

data "aws_partition" "current" {}

data "aws_region" "current" {}

# Logs ----------------------------------------------------------------------

# One log group for every process of the context, the proxies of Service
# Connect included. The streams start with the name of the process, such as
# api/ or relay/.
resource "aws_cloudwatch_log_group" "this" {
  name              = "/${var.environment}/${var.name}"
  retention_in_days = var.log_retention_days
}

# Events --------------------------------------------------------------------

# The queue of the context, and the rule that selects its events on the bus.
# They belong to the consumer, so the publisher does not know them (TPL-0013).
module "subscription" {
  source = "../event-subscription"
  count  = var.subscription == null ? 0 : 1

  name           = var.name
  event_bus_name = var.event_bus.name
  sources        = var.subscription.sources
  event_types    = var.subscription.event_types
}

# Network -------------------------------------------------------------------

# The group of the context: what its tasks reach beyond what every task
# reaches (the group of the services, from the module platform). So far, that
# is the database.
resource "aws_security_group" "this" {
  name        = "${var.environment}-${var.name}"
  description = "The tasks of the context ${var.name}."
  vpc_id      = var.vpc_id

  tags = {
    Name = "${var.environment}-${var.name}"
  }
}

resource "aws_vpc_security_group_egress_rule" "database" {
  security_group_id            = aws_security_group.this.id
  description                  = "PostgreSQL of the database."
  referenced_security_group_id = var.database.security_group_id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}

# The database accepts the tasks of this context. The rule lives here, with
# the context, so that a new context needs no change to the database.
resource "aws_vpc_security_group_ingress_rule" "database" {
  security_group_id            = var.database.security_group_id
  description                  = "PostgreSQL from the tasks of the context ${var.name}."
  referenced_security_group_id = aws_security_group.this.id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}

# Roles ---------------------------------------------------------------------

locals {
  # The permissions with which ECS starts a task of the context.
  execution_statements = concat(
    [
      # The token of ECR is for the whole registry; the action takes no
      # resource.
      {
        sid       = "SignInToTheRegistry"
        actions   = ["ecr:GetAuthorizationToken"]
        resources = ["*"]
      },
      {
        sid       = "PullTheImage"
        actions   = ["ecr:BatchCheckLayerAvailability", "ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer"]
        resources = [var.repository_arn]
      },
      {
        sid       = "WriteTheLogs"
        actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        resources = ["${aws_cloudwatch_log_group.this.arn}:*"]
      },
      {
        sid       = "ReadTheSecrets"
        actions   = ["secretsmanager:GetSecretValue"]
        resources = [var.database.secret_arn]
      },
    ],
    # A secret under a key of the team's own is unreadable without that key.
    # The key that AWS manages for Secrets Manager needs no statement.
    var.database.kms_key_arn == null ? [] : [{
      sid       = "DecryptTheSecrets"
      actions   = ["kms:Decrypt"]
      resources = [var.database.kms_key_arn]
    }],
  )

  # The permissions of the processes: what they call in AWS, and nothing
  # else.
  task_statements = concat(
    # The relay checks at its start that the bus exists, then puts each event
    # on it (backend/internal/platform/awsevents).
    var.publishes_events ? [{
      sid       = "PublishEvents"
      actions   = ["events:DescribeEventBus", "events:PutEvents"]
      resources = [var.event_bus.arn]
    }] : [],
    # The consumer receives the messages of its queue and deletes each that
    # it handled (backend/internal/platform/awsevents). The queue is encrypted
    # with a key of SQS, which needs no permission of KMS.
    var.subscription == null ? [] : [{
      sid       = "ConsumeEvents"
      actions   = ["sqs:DeleteMessage", "sqs:ReceiveMessage"]
      resources = [module.subscription[0].queue_arn]
    }],
    [for statement in var.iam_statements : {
      sid       = null
      actions   = statement.actions
      resources = statement.resources
    }],
  )
}

# The role with which ECS starts a task: it pulls the image, writes the logs,
# and reads the secret. The processes never get its permissions.
data "aws_iam_policy_document" "execution_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "execution" {
  dynamic "statement" {
    for_each = local.execution_statements

    content {
      sid       = statement.value.sid
      actions   = statement.value.actions
      resources = statement.value.resources
    }
  }
}

resource "aws_iam_role" "execution" {
  name               = "${var.environment}-${var.name}-execution"
  description        = "ECS starts the tasks of the context ${var.name} with this role."
  assume_role_policy = data.aws_iam_policy_document.execution_assume.json
}

resource "aws_iam_role_policy" "execution" {
  name   = "start-tasks"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.execution.json
}

# The role of the processes: the AWS SDK in a task gets its credentials. Only
# ECS of this account and Region may take it on, so that no other account can
# have ECS act with it (the confused deputy problem).
data "aws_iam_policy_document" "task_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }

    condition {
      test     = "ArnLike"
      variable = "aws:SourceArn"
      values   = ["arn:${data.aws_partition.current.partition}:ecs:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:*"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
}

resource "aws_iam_role" "task" {
  name               = "${var.environment}-${var.name}-task"
  description        = "The processes of the context ${var.name} call AWS with this role."
  assume_role_policy = data.aws_iam_policy_document.task_assume.json
}

data "aws_iam_policy_document" "task" {
  count = length(local.task_statements) > 0 ? 1 : 0

  dynamic "statement" {
    for_each = local.task_statements

    content {
      sid       = statement.value.sid
      actions   = statement.value.actions
      resources = statement.value.resources
    }
  }
}

resource "aws_iam_role_policy" "task" {
  count = length(local.task_statements) > 0 ? 1 : 0

  name   = "run-the-context"
  role   = aws_iam_role.task.id
  policy = data.aws_iam_policy_document.task[0].json
}

# Services ------------------------------------------------------------------

resource "aws_ecs_task_definition" "this" {
  for_each = local.processes

  family                   = "${var.environment}-${each.key}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = each.value.cpu
  memory                   = each.value.memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = var.cpu_architecture
  }

  container_definitions = jsonencode([local.containers[each.key]])
}

resource "aws_ecs_service" "this" {
  for_each = local.processes

  name                   = each.key
  cluster                = var.cluster_arn
  task_definition        = aws_ecs_task_definition.this[each.key].arn
  desired_count          = each.value.desired_count
  launch_type            = "FARGATE"
  enable_execute_command = false

  # The tasks carry the tags of their service, such as the default tags of
  # the environment, for the bill.
  enable_ecs_managed_tags = true
  propagate_tags          = "SERVICE"

  # In a private subnet, without a public address: a task reaches AWS through
  # the endpoints of the VPC, and nothing outside the VPC reaches the task.
  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.this.id, var.services_security_group_id]
    assign_public_ip = false
  }

  # A deployment whose tasks do not become healthy stops, and the service
  # goes back to the last deployment that worked.
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  # Every process is a client of Service Connect, so it can call the API of
  # any context by name. The API also serves under the name of the context:
  # the other services reach it at http://<name>:8080.
  service_connect_configuration {
    enabled   = true
    namespace = var.namespace_arn

    log_configuration {
      log_driver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.this.name
        awslogs-region        = data.aws_region.current.region
        awslogs-stream-prefix = each.value.log_prefix
      }
    }

    dynamic "service" {
      for_each = each.value.serves ? ["http"] : []

      content {
        port_name      = service.value
        discovery_name = var.name

        client_alias {
          dns_name = var.name
          port     = local.port
        }
      }
    }
  }
}
