# web

The web front end of one environment: the server of Next.js as an ECS service on Fargate, behind the one load balancer that the internet can reach (TPL-0014).

- The load balancer sits in the public subnets and serves HTTPS with TLS 1.2 or later. Port 80 only redirects to HTTPS.
- The tasks sit in the private subnets, without public addresses, and accept requests from the load balancer alone.
- The server calls the APIs of the contexts by name, such as `http://facility:8080`, as a client of Service Connect. The addresses arrive in `environment_variables`, so one image runs in every environment.
- The module requests a certificate for `domain_name` and validates it through the hosted zone, unless `certificate_arn` names one that exists. It points the domain name at the load balancer.
- The role of the task has no policy: the server calls no AWS service and holds no secret.
- The health check of the load balancer and of the container, `/healthz`, calls no API, so a context that is down does not take the site down.

## The one public entry

Only this module creates something that the internet can reach.
Trivy reports a load balancer that faces the internet (`AWS-0053`), and [.trivyignore.yaml](../../../.trivyignore.yaml) allows that finding for `infra/modules/web/main.tf` alone.
A public load balancer in any other file still fails `make check`.

## Not yet applied to AWS

`make check-all` applies this module to AWS Emulator with a certificate that the rehearsal requests itself, because the emulator never issues one. The default path, a certificate that ACM issues after the DNS validation, runs only in the tests of `tests/`, against a mock of the AWS provider.

The module keeps no access logs of the load balancer. Their bucket would be encrypted with keys that Amazon S3 manages, which is the only kind that a load balancer writes to, and Trivy requires a customer managed key for a bucket (`AWS-0132`). Decide on the logs, and on that finding, when you add them.

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
| [aws_acm_certificate.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/acm_certificate) | resource |
| [aws_acm_certificate_validation.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/acm_certificate_validation) | resource |
| [aws_cloudwatch_log_group.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_ecs_service.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_service) | resource |
| [aws_ecs_task_definition.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_task_definition) | resource |
| [aws_iam_role.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_lb.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lb) | resource |
| [aws_lb_listener.http](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lb_listener) | resource |
| [aws_lb_listener.https](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lb_listener) | resource |
| [aws_lb_target_group.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lb_target_group) | resource |
| [aws_route53_record.site](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route53_record) | resource |
| [aws_route53_record.validation](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route53_record) | resource |
| [aws_security_group.load_balancer](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_security_group.tasks](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_vpc_security_group_egress_rule.to_tasks](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_vpc_security_group_ingress_rule.from_load_balancer](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_ingress_rule) | resource |
| [aws_vpc_security_group_ingress_rule.http](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_ingress_rule) | resource |
| [aws_vpc_security_group_ingress_rule.https](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_ingress_rule) | resource |
| [aws_iam_policy_document.assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| certificate\_arn | The ARN of a certificate in AWS Certificate Manager that covers `domain_name`, in the Region of the load balancer. By default the module requests a certificate and validates it through the hosted zone; name one here to use a certificate that exists, such as a wildcard certificate of the team. | `string` | `null` | no |
| cluster\_arn | The ARN of the ECS cluster. | `string` | n/a | yes |
| cpu | The CPU units of a task; 1024 is one vCPU. Fargate accepts certain pairs of CPU and memory. | `number` | `256` | no |
| cpu\_architecture | The CPU architecture that the image is built for: `X86_64` or `ARM64`. | `string` | `"X86_64"` | no |
| deletion\_protection | Whether the load balancer refuses to be deleted. | `bool` | `true` | no |
| desired\_count | How many tasks run. | `number` | `1` | no |
| domain\_name | The domain name of the site, such as `stg.example.com`. The module requests a certificate for it and points the name at the load balancer. | `string` | n/a | yes |
| environment | The name of the environment, such as `stg`. Every resource is named after it. | `string` | n/a | yes |
| environment\_variables | The environment variables of the server, such as `FACILITY_API_URL = "http://facility:8080"`. They are not secret: the server of Next.js holds no secret. | `map(string)` | `{}` | no |
| hosted\_zone\_id | The ID of the public hosted zone in Amazon Route 53 that holds `domain_name`. The records of the name and of the validation of the certificate go there. | `string` | n/a | yes |
| image | The image of the frontend, with its tag, such as `111111111111.dkr.ecr.ap-northeast-1.amazonaws.com/web:0123456789abcdef`. | `string` | n/a | yes |
| log\_retention\_days | How many days CloudWatch Logs keeps the logs of the server. | `number` | `30` | no |
| memory | The memory of a task, in MiB. | `number` | `512` | no |
| namespace\_arn | The ARN of the Service Connect namespace of the cluster. Through it the server of Next.js calls the APIs of the contexts by name, such as `http://facility:8080`. | `string` | n/a | yes |
| public\_subnet\_ids | The IDs of the public subnets, for the load balancer. | `list(string)` | n/a | yes |
| repository\_arn | The ARN of the Amazon ECR repository of the image. The task may pull from this repository and from no other. | `string` | n/a | yes |
| services\_security\_group\_id | The ID of the security group that every task of the environment joins. It lets the task call the APIs of the contexts and reach the AWS services through the endpoints of the VPC. | `string` | n/a | yes |
| subnet\_ids | The IDs of the private subnets, for the tasks. | `list(string)` | n/a | yes |
| vpc\_id | The ID of the VPC. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| load\_balancer\_dns\_name | The DNS name of the load balancer, which the record of the domain name points at. |
| security\_group\_id | The ID of the security group of the tasks. |
| service\_name | The name of the ECS service. |
| url | The address of the site. |
<!-- END_TF_DOCS -->
