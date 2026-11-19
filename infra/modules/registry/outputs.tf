output "repository_urls" {
  description = "The URL of each repository, by name, such as `<account>.dkr.ecr.<region>.amazonaws.com/facility`. An image is this URL and a tag or a digest."
  value       = { for name, repository in aws_ecr_repository.this : name => repository.repository_url }
}

output "repository_arns" {
  description = "The ARN of each repository, by name: a context service may pull from the repository of its own image alone."
  value       = { for name, repository in aws_ecr_repository.this : name => repository.arn }
}
