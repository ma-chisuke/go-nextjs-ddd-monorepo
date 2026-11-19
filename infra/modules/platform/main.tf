# What every service of one environment shares (TPL-0014, TPL-0015): the ECS
# cluster, the namespace in which the services find each other, the event bus,
# and the security group that every task joins.
#
# The APIs of the contexts are private. A context calls another through ECS
# Service Connect, by the name of the other context, such as
# http://facility:8080, the address that it has in Docker Compose too. The
# proxy that ECS adds to each task resolves the name in the namespace of the
# environment and connects to a task of that context, so no load balancer and
# no DNS record stand in front of an API. Only the web front end has a public
# entry, in a module of its own.

locals {
  # The port on which every process of every context listens: HTTP_ADDR of
  # the services, by default (backend/internal/platform/httpserver).
  service_port = 8080
}

# The tasks run on Fargate (the module context-service), so the cluster has
# no instances to manage. Services that name no namespace of their own use
# the namespace of the environment.
resource "aws_ecs_cluster" "this" {
  name = var.environment

  setting {
    name  = "containerInsights"
    value = var.container_insights
  }

  service_connect_defaults {
    namespace = aws_service_discovery_http_namespace.this.arn
  }
}

# Service Connect needs a namespace of AWS Cloud Map. An HTTP namespace makes
# no DNS records: the proxy in each task learns the endpoints of the namespace
# from ECS, and only the services of the namespace can resolve them.
resource "aws_service_discovery_http_namespace" "this" {
  name        = var.environment
  description = "The services of the environment ${var.environment}, which find each other through ECS Service Connect."
}

module "event_bus" {
  source = "../event-bus"

  name = var.event_bus_name
}

# The services ---------------------------------------------------------------

# Every task of every service joins this group, besides the group of its
# context. It lets a task call the API of another context, and reach AWS
# through the endpoints of the VPC; nothing outside the VPC.
resource "aws_security_group" "services" {
  name        = "${var.environment}-services"
  description = "Every task of every service of the environment."
  vpc_id      = var.vpc_id

  tags = {
    Name = "${var.environment}-services"
  }
}

# Service Connect: the proxy in the task of a client connects to the proxy in
# a task of the server on the container port, which is the port that the
# server listens on.
resource "aws_vpc_security_group_ingress_rule" "services_from_services" {
  security_group_id            = aws_security_group.services.id
  description                  = "The APIs of the contexts, from the proxies of other tasks."
  referenced_security_group_id = aws_security_group.services.id
  from_port                    = local.service_port
  to_port                      = local.service_port
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "services_to_services" {
  security_group_id            = aws_security_group.services.id
  description                  = "The APIs of the contexts, through the proxy of the task."
  referenced_security_group_id = aws_security_group.services.id
  from_port                    = local.service_port
  to_port                      = local.service_port
  ip_protocol                  = "tcp"
}

# Amazon ECR, CloudWatch Logs, Secrets Manager, the ECS agent, EventBridge,
# SQS, and Amazon SES, through their interface endpoints.
resource "aws_vpc_security_group_egress_rule" "services_to_endpoints" {
  security_group_id            = aws_security_group.services.id
  description                  = "HTTPS to the interface endpoints of the AWS services."
  referenced_security_group_id = var.endpoints_security_group_id
  from_port                    = 443
  to_port                      = 443
  ip_protocol                  = "tcp"
}

# The layers of the images, through the gateway endpoint of Amazon S3.
resource "aws_vpc_security_group_egress_rule" "services_to_s3" {
  security_group_id = aws_security_group.services.id
  description       = "HTTPS to Amazon S3, through its gateway endpoint."
  prefix_list_id    = var.s3_prefix_list_id
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}
