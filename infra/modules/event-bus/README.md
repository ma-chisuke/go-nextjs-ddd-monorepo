# event-bus

The EventBridge event bus that carries the integration events of every bounded context (TPL-0013).
A publishing context puts its events on the bus through the relay of its outbox.
Each consuming context adds a rule of its own with the module [event-subscription](../event-subscription/README.md), so the publisher does not know its consumers.

The local environment applies this module to AWS Emulator (`infra/local/`), and the environments in AWS use the same module.

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
| [aws_cloudwatch_event_bus.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_bus) | resource |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| name | The name of the event bus. The relay of every publishing context gets it as EVENT\_BUS\_NAME. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| arn | The ARN of the event bus. |
| name | The name of the event bus. |
<!-- END_TF_DOCS -->
