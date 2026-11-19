output "plan_role_arn" {
  description = "The ARN of the read-only role, for the workflow that plans on pull requests."
  value       = aws_iam_role.plan.arn
}

output "deploy_role_arn" {
  description = "The ARN of the role that applies changes, for the workflow that deploys."
  value       = aws_iam_role.deploy.arn
}

output "plan_subject" {
  description = "The subject that a token must have to assume the read-only role."
  value       = local.plan_subject
}

output "deploy_subject" {
  description = "The subject that a token must have to assume the role that applies changes."
  value       = local.deploy_subject
}
