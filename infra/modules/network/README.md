# network

The network of one environment: a VPC with a public, a private, and a database subnet in each Availability Zone (TPL-0015).

| Subnet | What runs there | Route out of the VPC |
|---|---|---|
| public | The load balancer of the web front end, and the NAT gateways | The internet gateway |
| private | The tasks of the contexts and of the web front end | A NAT gateway |
| database | The Aurora cluster | None |

Only what sits in a public subnet can be reached from the internet, so the APIs of the contexts are private by where they run (TPL-0014).

- By default, every zone shares one NAT gateway, which costs the least. Set `single_nat_gateway` to `false` for an environment that must keep its outgoing traffic when a zone fails.
- A gateway endpoint for Amazon S3 keeps the layers of container images off the NAT gateway.
- The default security group of the VPC has no rules, and the flow logs of the VPC go to CloudWatch Logs.

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
| [aws_subnet.database](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_subnet.private](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_subnet.public](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_vpc.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc) | resource |
| [aws_vpc_endpoint.s3](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/vpc_endpoint) | resource |
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
| single\_nat\_gateway | Whether the private subnets of every zone share one NAT gateway. One gateway costs the least, and the outgoing traffic of every zone stops when its zone fails; `false` gives each zone a gateway of its own. | `bool` | `true` | no |

## Outputs

| Name | Description |
| ---- | ----------- |
| availability\_zones | The Availability Zones of the subnets, in the order given. |
| database\_subnet\_ids | The IDs of the database subnets, one for each zone, in the order of the zones: for the Aurora cluster. |
| private\_subnet\_ids | The IDs of the private subnets, one for each zone, in the order of the zones: for the tasks of the contexts and of the web front end. |
| public\_subnet\_ids | The IDs of the public subnets, one for each zone, in the order of the zones: for the load balancer of the web front end. |
| vpc\_id | The ID of the VPC. |
<!-- END_TF_DOCS -->
