# event-subscription

The subscription of one consuming context to events on the bus (TPL-0013): an SQS queue with a dead-letter queue, and an EventBridge rule that selects events by their `source` and `type` into the queue.
Everything here belongs to the consumer, so adding a consumer changes nothing in the publisher.

The queue receives the CloudEvents envelope alone, as the publisher wrote it (the input path `$.detail`).
A message that the consumer could not handle `max_receive_count` times moves to the dead-letter queue, so it does not hold up the rest and is not lost.

The local environment applies this module to AWS Emulator (`infra/local/`). The environments in AWS will use the same module, from the infrastructure milestone on (TPL-0015).

terraform-docs writes the lists of inputs, outputs, and resources below from the code. The descriptions in the code are in English, so the lists are in this English version only.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.17 |
| aws | >= 6.0 |

## Providers

| Name | Version |
| ---- | ------- |
| aws | >= 6.0 |

## Modules

No modules.

## Resources

| Name | Type |
| ---- | ---- |
| [aws_cloudwatch_event_rule.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_rule) | resource |
| [aws_cloudwatch_event_target.queue](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_target) | resource |
| [aws_sqs_queue.dead_letter](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/sqs_queue) | resource |
| [aws_sqs_queue.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/sqs_queue) | resource |
| [aws_caller_identity.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [aws_iam_policy_document.deliver](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_partition.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/partition) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| dead\_letter\_retention\_seconds | How long the dead-letter queue keeps a message for someone to look at. The default is 14 days, the longest that SQS allows. | `number` | `1209600` | no |
| event\_bus\_name | The name of the event bus that carries the events. | `string` | n/a | yes |
| event\_types | The types of the events to receive: the CloudEvents `type` of each event, such as `reservation.ReservationConfirmed.v1`. | `list(string)` | n/a | yes |
| max\_receive\_count | How often the queue shows a message to the consumer before it moves the message to the dead-letter queue. | `number` | `5` | no |
| name | The name of the consuming context, such as `notification`. The queue is named after it with the suffix `-events`, and its dead-letter queue with the suffix `-events-dlq`. | `string` | n/a | yes |
| sources | The sources of the events to receive: the CloudEvents `source` of each publishing context, such as `/contexts/reservation`. | `list(string)` | n/a | yes |
| visibility\_timeout\_seconds | How long a received message stays hidden from other receives. It must exceed the time that the consumer gives the handling of one event (25 seconds by default, in `backend/internal/platform/awsevents`), or the event comes again while it is still being handled. | `number` | `60` | no |

## Outputs

| Name | Description |
| ---- | ----------- |
| dead\_letter\_queue\_arn | The ARN of the dead-letter queue, for an alarm on messages that arrive there. |
| dead\_letter\_queue\_url | The URL of the dead-letter queue. |
| queue\_arn | The ARN of the queue, for the policy of the role that reads it. |
| queue\_url | The URL of the queue. The worker of the consuming context gets it as EVENTS\_QUEUE\_URL. |
<!-- END_TF_DOCS -->
