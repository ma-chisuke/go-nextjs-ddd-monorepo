output "cluster_arn" {
  description = "The ARN of the ECS cluster, in which every service of the environment runs."
  value       = aws_ecs_cluster.this.arn
}

output "cluster_name" {
  description = "The name of the ECS cluster."
  value       = aws_ecs_cluster.this.name
}

output "namespace_arn" {
  description = "The ARN of the namespace of AWS Cloud Map in which the services find each other through ECS Service Connect."
  value       = aws_service_discovery_http_namespace.this.arn
}

output "event_bus" {
  description = "The name and the ARN of the event bus that carries the integration events of every context."
  value = {
    name = module.event_bus.name
    arn  = module.event_bus.arn
  }
}

output "services_security_group_id" {
  description = "The ID of the security group that every task of every service joins: it may call the APIs of the contexts, and reach AWS through the endpoints of the VPC."
  value       = aws_security_group.services.id
}
