output "api_url" {
  description = "The URL at which the other services call the API of the context, such as `http://reservation:8080`, through ECS Service Connect; null when the context has no API."
  value       = var.api == null ? null : "http://${var.name}:${local.port}"
}

output "security_group_id" {
  description = "The ID of the security group of the context, which its tasks join besides the group of the services. A rule that only this context needs, such as one to a service outside AWS, belongs here."
  value       = aws_security_group.this.id
}

output "task_role_arn" {
  description = "The ARN of the role with which the processes of the context call AWS."
  value       = aws_iam_role.task.arn
}

output "service_names" {
  description = "The names of the ECS services of the context: `<name>` for the API, and `<name>-<key>` for each worker."
  value       = sort([for service in aws_ecs_service.this : service.name])
}

output "log_group_name" {
  description = "The name of the CloudWatch log group of every process of the context, such as `/stg/reservation`."
  value       = aws_cloudwatch_log_group.this.name
}

output "events_queue_url" {
  description = "The URL of the queue of the events that the context consumes, which its processes get as EVENTS_QUEUE_URL; null without a subscription."
  value       = one(module.subscription[*].queue_url)
}
