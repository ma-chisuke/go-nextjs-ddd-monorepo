output "endpoint" {
  description = "The address of the writer of the cluster."
  value       = aws_rds_cluster.this.endpoint
}

output "port" {
  description = "The port of PostgreSQL."
  value       = local.port
}

output "database_name" {
  description = "The name of the database that every context shares."
  value       = aws_rds_cluster.this.database_name
}

output "security_group_id" {
  description = "The ID of the security group of the cluster. A context lets its tasks connect with an ingress rule in this group, from the security group of its tasks, on the port of PostgreSQL."
  value       = aws_security_group.cluster.id
}

output "runtime_secret_arns" {
  description = "The ARN of the secret of the runtime role of each context, by context. The key `url` of the secret is the `DATABASE_URL` of the service of that context: its task definition reads it with `valueFrom = \"<ARN>:url::\"`, and its execution role needs `secretsmanager:GetSecretValue` on the ARN."
  value       = { for context in var.contexts : context => aws_secretsmanager_secret.role["${context}/runtime"].arn }
}

output "db_task" {
  description = "What `aws ecs run-task` needs to run the task that prepares the database, once at each deploy, before the services start: the task definition, the private subnets and the security group of its network configuration (awsvpc, without a public address), and the name of its container."
  value = {
    task_definition_arn = aws_ecs_task_definition.db.arn
    subnet_ids          = var.task_subnet_ids
    security_group_id   = aws_security_group.task.id
    container_name      = local.container_name
  }
}
