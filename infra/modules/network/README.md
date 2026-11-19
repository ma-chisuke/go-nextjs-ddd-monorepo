# network

The network of one environment: a VPC with a public, a private, and a database subnet in each Availability Zone (TPL-0015).

| Subnet | What runs there | Route out of the VPC |
|---|---|---|
| public | The load balancer of the web front end | The internet gateway |
| private | The tasks of the contexts and of the web front end | None by default; the AWS services through VPC endpoints |
| database | The Aurora cluster | None |

Only what sits in a public subnet can be reached from the internet, so the APIs of the contexts are private by where they run (TPL-0014).

## How the tasks reach AWS

By default, nothing in the private subnets can reach the internet.
The tasks reach the AWS services that they call through VPC endpoints (AWS PrivateLink): a gateway endpoint for Amazon S3, and an interface endpoint for each service in `interface_endpoints`.
The traffic stays inside the VPC, and the security group of a task allows no address outside it: it allows HTTPS to the security group of the endpoints (`endpoints_security_group_id`) and to the prefix list of Amazon S3 (`s3_prefix_list_id`).

AWS describes a NAT gateway as the easiest way to reach its services, and names its cost: a NAT gateway cannot limit the destinations that a task talks to.
The security pillar of the AWS Well-Architected Framework recommends VPC endpoints as the default way to reach managed services.
The template follows that recommendation.

- An interface endpoint is charged by the hour for each zone. `interface_endpoints_in_every_zone = false` keeps one interface for each endpoint, which costs less and fails with its zone; it suits an environment such as stg.
- A context that calls a service outside AWS needs a route to the internet: set `nat_gateways` to `single` or `per-zone`, and allow the traffic in the security group of that context.
- A service that the tasks call and that is not in `interface_endpoints` cannot be reached. Add it to the list.
- The default security group of the VPC has no rules, and the flow logs of the VPC go to CloudWatch Logs.

Not yet applied to AWS: the service name of the endpoint for the API of Amazon SES, `email`, follows the documentation of Amazon SES, which does not spell the name out. Check it with `aws ec2 describe-vpc-endpoint-services` when you first apply the module.

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
| [aws_cloudwatch_log_group.flow](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_default_security_group.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/default_security_group) | resource |
| [aws_eip.nat](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eip) | resource |
| [aws_flow_log.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/flow_log) | resource |
| [aws_iam_role.flow](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.flow](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_internet_gateway.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/internet_gateway) | resource |
| [aws_nat_gateway.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/nat_gateway) | resource |
| [aws_route.private_internet](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route) | resource |
| [aws_route.public_internet](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route) | resource |
| [aws_route_table.database](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table) | resource |
| [aws_route_table.private](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table) | resource |
| [aws_route_table.public](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table) | resource |
| [aws_route_table_association.database](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table_association) | resource |
| [aws_route_table_association.private](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table_association) | resource |
| [aws_route_table_association.public](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table_association) | resource |
| [aws_security_group.endpoints](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_subnet.database](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_subnet.private](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_subnet.public](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_vpc.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc) | resource |
| [aws_vpc_endpoint.interface](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_endpoint) | resource |
| [aws_vpc_endpoint.s3](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_endpoint) | resource |
| [aws_vpc_security_group_ingress_rule.endpoints_https](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_security_group_ingress_rule) | resource |
| [aws_iam_policy_document.flow_assume](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.flow_write](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| availability\_zones | The Availability Zones of the subnets, such as `["ap-northeast-1a", "ap-northeast-1c"]`. Each zone gets one public, one private, and one database subnet. Two zones are the least that a load balancer and an Aurora cluster accept. | `list(string)` | n/a | yes |
| cidr\_block | The address range of the VPC. It needs a prefix of /20 or shorter: the module cuts it into 16 equal parts and gives one part to each subnet. | `string` | `"10.0.0.0/16"` | no |
| environment | The name of the environment, such as `stg`. Every resource is named after it. | `string` | n/a | yes |
| flow\_log\_retention\_days | How many days CloudWatch Logs keeps the flow logs of the VPC. | `number` | `30` | no |
| interface\_endpoints | The AWS services that the tasks in the private subnets reach through an interface endpoint (AWS PrivateLink), as the part of the service name after the Region: `ecr.api` stands for `com.amazonaws.<region>.ecr.api`. The default is what the services of the template call: Amazon ECR and CloudWatch Logs and Secrets Manager to start a task, the ECS agent for Service Connect, and EventBridge, SQS, and the API of Amazon SES for the events and the mail. | `set(string)` | ```[ "ecr.api", "ecr.dkr", "logs", "secretsmanager", "ecs-agent", "events", "sqs", "email" ]``` | no |
| interface\_endpoints\_in\_every\_zone | Whether each interface endpoint has a network interface in every zone. An endpoint is charged for each zone by the hour; `false` keeps one interface, in the first zone, which the other zones reach across zones and which fails with its zone. | `bool` | `true` | no |
| nat\_gateways | The NAT gateways that give the private subnets a route to the internet: `none`, `single` (one gateway that every zone shares), or `per-zone`. The default is `none`: the tasks reach AWS through the endpoints and cannot reach the internet. A context that calls a service outside AWS needs a NAT gateway, and a rule in its security group that allows the traffic. | `string` | `"none"` | no |

## Outputs

| Name | Description |
| ---- | ----------- |
| availability\_zones | The Availability Zones of the subnets, in the order given. |
| database\_subnet\_ids | The IDs of the database subnets, one for each zone, in the order of the zones: for the Aurora cluster. |
| endpoints\_security\_group\_id | The ID of the security group of the interface endpoints. A task reaches the AWS services when its security group allows HTTPS to this group. |
| private\_subnet\_ids | The IDs of the private subnets, one for each zone, in the order of the zones: for the tasks of the contexts and of the web front end. |
| public\_subnet\_ids | The IDs of the public subnets, one for each zone, in the order of the zones: for the load balancer of the web front end. |
| s3\_prefix\_list\_id | The ID of the prefix list of Amazon S3 in this Region. A task pulls the layers of its image when its security group allows HTTPS to this prefix list. |
| vpc\_id | The ID of the VPC. |
<!-- END_TF_DOCS -->
