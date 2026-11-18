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

variable "single_nat_gateway" {
  description = "Whether the private subnets of every zone share one NAT gateway. One gateway costs the least, and the outgoing traffic of every zone stops when its zone fails; `false` gives each zone a gateway of its own."
  type        = bool
  default     = true
}

variable "flow_log_retention_days" {
  description = "How many days CloudWatch Logs keeps the flow logs of the VPC."
  type        = number
  default     = 30
}
