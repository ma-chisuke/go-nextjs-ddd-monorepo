variable "environment" {
  description = "The name of the environment, such as `stg`. Every resource is named after it."
  type        = string
}

variable "cidr_block" {
  description = "The address range of the VPC. It needs a prefix of /20 or shorter: the module cuts it into 16 equal parts and gives one part to each subnet."
  type        = string
  default     = "10.0.0.0/16"

  validation {
    condition     = can(cidrsubnet(var.cidr_block, 4, 15)) && tonumber(split("/", var.cidr_block)[1]) <= 20
    error_message = "The address range must be an IPv4 CIDR block with a prefix of /20 or shorter, such as 10.0.0.0/16."
  }
}

variable "availability_zones" {
  description = "The Availability Zones of the subnets, such as `[\"ap-northeast-1a\", \"ap-northeast-1c\"]`. Each zone gets one public, one private, and one database subnet. Two zones are the least that a load balancer and an Aurora cluster accept."
  type        = list(string)

  validation {
    condition     = length(var.availability_zones) >= 2 && length(var.availability_zones) <= 4 && length(distinct(var.availability_zones)) == length(var.availability_zones)
    error_message = "Name two to four different Availability Zones."
  }
}

variable "interface_endpoints" {
  description = "The AWS services that the tasks in the private subnets reach through an interface endpoint (AWS PrivateLink), as the part of the service name after the Region: `ecr.api` stands for `com.amazonaws.<region>.ecr.api`. The default is what the services of the template call: Amazon ECR and CloudWatch Logs and Secrets Manager to start a task, the ECS agent for Service Connect, and EventBridge, SQS, and the API of Amazon SES for the events and the mail."
  type        = set(string)
  default     = ["ecr.api", "ecr.dkr", "logs", "secretsmanager", "ecs-agent", "events", "sqs", "email"]
}

variable "interface_endpoints_in_every_zone" {
  description = "Whether each interface endpoint has a network interface in every zone. An endpoint is charged for each zone by the hour; `false` keeps one interface, in the first zone, which the other zones reach across zones and which fails with its zone."
  type        = bool
  default     = true
}

variable "nat_gateways" {
  description = "The NAT gateways that give the private subnets a route to the internet: `none`, `single` (one gateway that every zone shares), or `per-zone`. The default is `none`: the tasks reach AWS through the endpoints and cannot reach the internet. A context that calls a service outside AWS needs a NAT gateway, and a rule in its security group that allows the traffic."
  type        = string
  default     = "none"

  validation {
    condition     = contains(["none", "single", "per-zone"], var.nat_gateways)
    error_message = "Choose none, single, or per-zone."
  }
}

variable "flow_log_retention_days" {
  description = "How many days CloudWatch Logs keeps the flow logs of the VPC."
  type        = number
  default     = 30
}
