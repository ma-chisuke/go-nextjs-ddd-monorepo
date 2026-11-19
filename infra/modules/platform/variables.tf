variable "environment" {
  description = "The name of the environment, such as `stg`. The cluster and the namespace are named after it, and the security group of the services with the suffix `-services`."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]*$", var.environment))
    error_message = "The name of the environment is in lowercase letters, digits, and hyphens, and starts with a letter, such as stg."
  }
}

variable "vpc_id" {
  description = "The ID of the VPC of the environment: `vpc_id` of the module network."
  type        = string
}

variable "endpoints_security_group_id" {
  description = "The ID of the security group of the interface endpoints: `endpoints_security_group_id` of the module network. The services reach the AWS services through them."
  type        = string
}

variable "s3_prefix_list_id" {
  description = "The ID of the prefix list of Amazon S3: `s3_prefix_list_id` of the module network. The services pull the layers of their images from Amazon S3."
  type        = string
}

variable "event_bus_name" {
  description = "The name of the event bus that carries the integration events of every context. The default is the name that the local environment uses (`infra/local/`)."
  type        = string
  default     = "contexts"
}

variable "container_insights" {
  description = "CloudWatch Container Insights for the cluster: `enabled`, `enhanced` (with the metrics of each container too), or `disabled`. Container Insights is charged for the metrics and the logs that it writes; an environment such as stg can turn it off."
  type        = string
  default     = "enabled"

  validation {
    condition     = contains(["enabled", "enhanced", "disabled"], var.container_insights)
    error_message = "Choose enabled, enhanced, or disabled."
  }
}
