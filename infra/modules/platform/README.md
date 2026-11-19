# platform

What every service of one environment shares (TPL-0014, TPL-0015): the ECS cluster, the namespace in which the services find each other, the event bus of the integration events, and the security group that every task joins.

| Resource | Name | Use |
|---|---|---|
| ECS cluster | `<environment>` | Every service of the environment, on Fargate |
| HTTP namespace of AWS Cloud Map | `<environment>` | The names under which the services call each other through ECS Service Connect |
| EventBridge event bus | `event_bus_name`, `contexts` by default | The integration events of every context (TPL-0013), with the module [event-bus](../event-bus/README.md) |
| Security group | `<environment>-services` | Every task of every service |

## How a context calls another

The APIs of the contexts are private, and no load balancer stands in front of them.
A context calls another through ECS Service Connect, by the name of the other context: Reservation calls `http://facility:8080`, the address that it has in Docker Compose too.
ECS adds a proxy to each task of a service that enables Service Connect. The proxy resolves the name in the namespace of the environment and connects to a task of that context, in round robin, with retries and passive health checks.

The security group `<environment>-services` allows only the traffic that every task needs:

- TCP on 8080, the port of every process, from and to the tasks of the group. The proxy of a client connects to the proxy of the server on the container port, which is the port that the server listens on.
- HTTPS to the security group of the interface endpoints and to the prefix list of Amazon S3, outputs of the module [network](../network/README.md). The tasks reach AWS through them.

No rule names an address range, so a task cannot reach anything outside the VPC.
A context adds what it needs alone, such as its database, in the security group of its own (the module [context-service](../context-service/README.md)).

- The cluster has Container Insights on. It is charged for the metrics and the logs that it writes; `container_insights = "disabled"` turns it off, for an environment such as stg.
- The namespace makes no DNS records: the proxies learn the endpoints from ECS, and only the services of the namespace can resolve them.

Not yet applied to AWS. AWS Emulator accepts the module, wired with the modules network and context-service, but it starts no task, so it does not show that the proxies resolve the names.

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

| Name | Source | Version |
| ---- | ------ | ------- |
| event\_bus | ../event-bus | n/a |

## Resources

| Name | Type |
| ---- | ---- |
| [aws_ecs_cluster.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_cluster) | resource |
| [aws_security_group.services](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_service_discovery_http_namespace.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/service_discovery_http_namespace) | resource |
| [aws_vpc_security_group_egress_rule.services_to_endpoints](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_vpc_security_group_egress_rule.services_to_s3](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_vpc_security_group_egress_rule.services_to_services](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_egress_rule) | resource |
| [aws_vpc_security_group_ingress_rule.services_from_services](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_ingress_rule) | resource |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| container\_insights | CloudWatch Container Insights for the cluster: `enabled`, `enhanced` (with the metrics of each container too), or `disabled`. Container Insights is charged for the metrics and the logs that it writes; an environment such as stg can turn it off. | `string` | `"enabled"` | no |
| endpoints\_security\_group\_id | The ID of the security group of the interface endpoints: `endpoints_security_group_id` of the module network. The services reach the AWS services through them. | `string` | n/a | yes |
| environment | The name of the environment, such as `stg`. The cluster and the namespace are named after it, and the security group of the services with the suffix `-services`. | `string` | n/a | yes |
| event\_bus\_name | The name of the event bus that carries the integration events of every context. The default is the name that the local environment uses (`infra/local/`). | `string` | `"contexts"` | no |
| s3\_prefix\_list\_id | The ID of the prefix list of Amazon S3: `s3_prefix_list_id` of the module network. The services pull the layers of their images from Amazon S3. | `string` | n/a | yes |
| vpc\_id | The ID of the VPC of the environment: `vpc_id` of the module network. | `string` | n/a | yes |

## Outputs

| Name | Description |
| ---- | ----------- |
| cluster\_arn | The ARN of the ECS cluster, in which every service of the environment runs. |
| cluster\_name | The name of the ECS cluster. |
| event\_bus | The name and the ARN of the event bus that carries the integration events of every context. |
| namespace\_arn | The ARN of the namespace of AWS Cloud Map in which the services find each other through ECS Service Connect. |
| services\_security\_group\_id | The ID of the security group that every task of every service joins: it may call the APIs of the contexts, and reach AWS through the endpoints of the VPC. |
<!-- END_TF_DOCS -->
