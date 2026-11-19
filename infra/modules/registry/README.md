# registry

The Amazon ECR repositories of the images that an environment runs: one repository for each image, such as the image of each context and that of the web front end (TPL-0014).

- Tags are immutable. A tag names one image for good, so a task definition that names a tag runs the image that was reviewed, and a push cannot change what a rollback goes back to.
- ECR scans each image when it is pushed.
- A lifecycle policy keeps the newest `kept_images` images of each repository, 30 by default, tagged or not, and expires the rest. An image that a task definition still names can expire too, and its service then cannot start a task, so keep more images than a rollback can need.

A context service may pull from the repository of its own image alone: the module [context-service](../context-service/README.md) takes the ARN of that repository from `repository_arns`.

The tests in `tests/` plan the module against a mock of the AWS provider, so `make check-all` runs them without an AWS account (`validate-terraform`).

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
| [aws_ecr_lifecycle_policy.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecr_lifecycle_policy) | resource |
| [aws_ecr_repository.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecr_repository) | resource |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| kept\_images | How many images each repository keeps. When a push makes one more, the lifecycle policy expires the oldest. Keep enough for a rollback: an image that a task definition still names can expire, and its service then fails to start a task. | `number` | `30` | no |
| repositories | The names of the repositories, one for each image: each context, the web front end, and any other image that the environments run, such as `["facility", "reservation", "notification", "web"]`. A name is in lowercase letters, digits, and hyphens, and starts with a letter. | `set(string)` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| repository\_arns | The ARN of each repository, by name: a context service may pull from the repository of its own image alone. |
| repository\_urls | The URL of each repository, by name, such as `<account>.dkr.ecr.<region>.amazonaws.com/facility`. An image is this URL and a tag or a digest. |
<!-- END_TF_DOCS -->
