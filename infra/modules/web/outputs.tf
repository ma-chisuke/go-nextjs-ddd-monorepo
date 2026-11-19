output "url" {
  description = "The address of the site."
  value       = "https://${var.domain_name}"
}

output "load_balancer_dns_name" {
  description = "The DNS name of the load balancer, which the record of the domain name points at."
  value       = aws_lb.this.dns_name
}

output "security_group_id" {
  description = "The ID of the security group of the tasks."
  value       = aws_security_group.tasks.id
}

output "service_name" {
  description = "The name of the ECS service."
  value       = aws_ecs_service.this.name
}
