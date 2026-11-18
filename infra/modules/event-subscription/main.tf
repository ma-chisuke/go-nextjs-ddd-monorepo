# The subscription of one consuming context to events on the bus (TPL-0013):
# its queue, with a dead-letter queue, and the rule that selects the events
# into the queue. Everything here belongs to the consumer, so adding a
# consumer changes nothing in the publisher.

locals {
  queue_name = "${var.name}-events"

  # The ARN of the queue, spelled out: the queue and its dead-letter queue
  # name each other, and the policy of the queue names the queue. As arguments
  # of the queues themselves, these cost no extra resource: the provider waits
  # about half a minute after every change to a queue, and a separate policy
  # resource would wait once more.
  queue_arn = "arn:${data.aws_partition.current.partition}:sqs:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:${local.queue_name}"
}

data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

data "aws_partition" "current" {}

# A message that the consumer could not handle max_receive_count times waits
# here, so that it does not hold up the rest and is not lost.
resource "aws_sqs_queue" "dead_letter" {
  name                      = "${local.queue_name}-dlq"
  message_retention_seconds = var.dead_letter_retention_seconds
  sqs_managed_sse_enabled   = true

  # Only the queue of this subscription may use it as its dead-letter queue.
  redrive_allow_policy = jsonencode({
    redrivePermission = "byQueue"
    sourceQueueArns   = [local.queue_arn]
  })
}

resource "aws_sqs_queue" "this" {
  name                       = local.queue_name
  visibility_timeout_seconds = var.visibility_timeout_seconds
  sqs_managed_sse_enabled    = true

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dead_letter.arn
    maxReceiveCount     = var.max_receive_count
  })

  policy = data.aws_iam_policy_document.deliver.json
}

# EventBridge may send to the queue, and only for the rule of this
# subscription. Without this statement EventBridge drops the events, in AWS
# and in AWS Emulator alike.
data "aws_iam_policy_document" "deliver" {
  statement {
    sid       = "DeliverFromTheRule"
    actions   = ["sqs:SendMessage"]
    resources = [local.queue_arn]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }

    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_cloudwatch_event_rule.this.arn]
    }
  }
}

# On the bus, the `source` and the `detail-type` of an entry are the `source`
# and the `type` of the CloudEvents envelope (backend/internal/platform/awsevents).
resource "aws_cloudwatch_event_rule" "this" {
  name           = local.queue_name
  description    = "The events that the context ${var.name} consumes."
  event_bus_name = var.event_bus_name

  event_pattern = jsonencode({
    source      = var.sources
    detail-type = var.event_types
  })
}

# The queue receives the `detail` alone, which is the envelope as the
# publisher wrote it, without the wrapping of EventBridge.
resource "aws_cloudwatch_event_target" "queue" {
  rule           = aws_cloudwatch_event_rule.this.name
  event_bus_name = var.event_bus_name
  target_id      = "queue"
  arn            = aws_sqs_queue.this.arn
  input_path     = "$.detail"
}
