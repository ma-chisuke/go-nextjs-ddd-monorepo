output "queue_url" {
  description = "The URL of the queue. The worker of the consuming context gets it as EVENTS_QUEUE_URL."
  value       = aws_sqs_queue.this.url
}

output "queue_arn" {
  description = "The ARN of the queue, for the policy of the role that reads it."
  value       = aws_sqs_queue.this.arn
}

output "dead_letter_queue_url" {
  description = "The URL of the dead-letter queue."
  value       = aws_sqs_queue.dead_letter.url
}

output "dead_letter_queue_arn" {
  description = "The ARN of the dead-letter queue, for an alarm on messages that arrive there."
  value       = aws_sqs_queue.dead_letter.arn
}
