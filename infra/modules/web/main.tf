# The web front end of an environment (TPL-0014): the server of Next.js as an
# ECS service on Fargate, behind the one load balancer that the internet can
# reach. Everything else of the system is private. The browser talks to this
# server alone, and the server calls the APIs of the contexts by name through
# Service Connect.
#
# Only this module creates a public entry. Trivy reports a load balancer
# that faces the internet, and .trivyignore.yaml allows that finding for the
# files of this module alone, so a public load balancer anywhere else still
# fails `make check`.

locals {
  name           = "${var.environment}-web"
  container_name = "web"
  port           = 3000
}

data "aws_region" "current" {}

# TLS -----------------------------------------------------------------------

# Unless the caller names a certificate, the module requests one.
resource "aws_acm_certificate" "this" {
  count = var.certificate_arn == null ? 1 : 0

  domain_name       = var.domain_name
  validation_method = "DNS"

  # A new certificate is issued and attached before the old one goes.
  lifecycle {
    create_before_destroy = true
  }
}

# ACM proves the ownership of the name by a record that it asks for. The
# certificate holds one name, so it asks for one record.
locals {
  validation = var.certificate_arn == null ? one(aws_acm_certificate.this[0].domain_validation_options) : null

  # The certificate of the listener: the one given, or the one requested,
  # once ACM has issued it.
  certificate_arn = var.certificate_arn == null ? aws_acm_certificate_validation.this[0].certificate_arn : var.certificate_arn
}

resource "aws_route53_record" "validation" {
  count = var.certificate_arn == null ? 1 : 0

  zone_id         = var.hosted_zone_id
  name            = local.validation.resource_record_name
  type            = local.validation.resource_record_type
  records         = [local.validation.resource_record_value]
  ttl             = 60
  allow_overwrite = true
}

resource "aws_acm_certificate_validation" "this" {
  count = var.certificate_arn == null ? 1 : 0

  certificate_arn         = aws_acm_certificate.this[0].arn
  validation_record_fqdns = [aws_route53_record.validation[0].fqdn]
}

# Load balancer -------------------------------------------------------------

resource "aws_security_group" "load_balancer" {
  name        = "${local.name}-lb"
  description = "The load balancer of the web front end: the public entry."
  vpc_id      = var.vpc_id

  tags = {
    Name = "${local.name}-lb"
  }
}

resource "aws_vpc_security_group_ingress_rule" "https" {
  security_group_id = aws_security_group.load_balancer.id
  description       = "HTTPS from the internet."
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}

# Port 80 only answers with a redirect to HTTPS.
resource "aws_vpc_security_group_ingress_rule" "http" {
  security_group_id = aws_security_group.load_balancer.id
  description       = "HTTP from the internet, to redirect it to HTTPS."
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 80
  to_port           = 80
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "to_tasks" {
  security_group_id            = aws_security_group.load_balancer.id
  description                  = "To the tasks of the web front end."
  referenced_security_group_id = aws_security_group.tasks.id
  from_port                    = local.port
  to_port                      = local.port
  ip_protocol                  = "tcp"
}

resource "aws_lb" "this" {
  name               = local.name
  load_balancer_type = "application"
  internal           = false
  subnets            = var.public_subnet_ids
  security_groups    = [aws_security_group.load_balancer.id]

  drop_invalid_header_fields = true
  enable_deletion_protection = var.deletion_protection
}

resource "aws_lb_target_group" "this" {
  name        = local.name
  vpc_id      = var.vpc_id
  target_type = "ip"
  port        = local.port
  protocol    = "HTTP"

  # A task that stops gets this long to finish the requests that it has.
  deregistration_delay = 30

  # The health check of the frontend calls no API: a context that is down
  # must not take the site out of the load balancer.
  health_check {
    path                = "/healthz"
    matcher             = "200"
    interval            = 10
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.this.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = local.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this.arn
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_route53_record" "site" {
  zone_id = var.hosted_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_lb.this.dns_name
    zone_id                = aws_lb.this.zone_id
    evaluate_target_health = true
  }
}

# Tasks ---------------------------------------------------------------------

# The tasks accept requests from the load balancer alone. What they may call
# is in the security group of the services, which they also join.
resource "aws_security_group" "tasks" {
  name        = local.name
  description = "The tasks of the web front end."
  vpc_id      = var.vpc_id

  tags = {
    Name = local.name
  }
}

resource "aws_vpc_security_group_ingress_rule" "from_load_balancer" {
  security_group_id            = aws_security_group.tasks.id
  description                  = "From the load balancer."
  referenced_security_group_id = aws_security_group.load_balancer.id
  from_port                    = local.port
  to_port                      = local.port
  ip_protocol                  = "tcp"
}

resource "aws_cloudwatch_log_group" "this" {
  name              = "/${var.environment}/web"
  retention_in_days = var.log_retention_days
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# What ECS needs to start a task: the image and the log stream.
data "aws_iam_policy_document" "execution" {
  statement {
    sid       = "SignInToTheRegistry"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  statement {
    sid = "PullTheImage"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:GetDownloadUrlForLayer",
    ]
    resources = [var.repository_arn]
  }

  statement {
    sid       = "WriteTheLogs"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.this.arn}:*"]
  }
}

resource "aws_iam_role" "execution" {
  name               = "${local.name}-execution"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

resource "aws_iam_role_policy" "execution" {
  name   = "start-the-task"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.execution.json
}

# The server calls no AWS service, so its role has no policy.
resource "aws_iam_role" "task" {
  name               = "${local.name}-task"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

resource "aws_ecs_task_definition" "this" {
  family                   = local.name
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = var.cpu_architecture
  }

  container_definitions = jsonencode([
    {
      name      = local.container_name
      image     = var.image
      essential = true

      readonlyRootFilesystem = true
      linuxParameters = {
        capabilities = {
          drop = ["ALL"]
        }
      }

      portMappings = [
        {
          name          = "http"
          containerPort = local.port
          protocol      = "tcp"
          appProtocol   = "http"
        }
      ]

      environment = [
        for name in sort(keys(var.environment_variables)) : {
          name  = name
          value = var.environment_variables[name]
        }
      ]

      # The command of the image's HEALTHCHECK, which ECS does not read
      # (frontend/Dockerfile).
      healthCheck = {
        command     = ["CMD", "/nodejs/bin/node", "/app/healthcheck.mjs"]
        interval    = 10
        timeout     = 3
        retries     = 3
        startPeriod = 30
      }

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.this.name
          awslogs-region        = data.aws_region.current.region
          awslogs-stream-prefix = local.container_name
        }
      }
    }
  ])
}

resource "aws_ecs_service" "this" {
  name            = "web"
  cluster         = var.cluster_arn
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  propagate_tags         = "SERVICE"
  enable_execute_command = false

  # A deployment that does not become healthy is rolled back.
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.tasks.id, var.services_security_group_id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.this.arn
    container_name   = local.container_name
    container_port   = local.port
  }

  # A client of Service Connect: the server resolves the names of the
  # contexts, and offers no name itself.
  service_connect_configuration {
    enabled   = true
    namespace = var.namespace_arn
  }

  # The target group must belong to a listener before a service may use it.
  depends_on = [aws_lb_listener.https]
}
