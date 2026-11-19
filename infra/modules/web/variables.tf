variable "environment" {
  description = "The name of the environment, such as `stg`. Every resource is named after it."
  type        = string
}

variable "image" {
  description = "The image of the frontend, with its tag, such as `111111111111.dkr.ecr.ap-northeast-1.amazonaws.com/web:0123456789abcdef`."
  type        = string
}

variable "repository_arn" {
  description = "The ARN of the Amazon ECR repository of the image. The task may pull from this repository and from no other."
  type        = string
}

variable "cluster_arn" {
  description = "The ARN of the ECS cluster."
  type        = string
}

variable "namespace_arn" {
  description = "The ARN of the Service Connect namespace of the cluster. Through it the server of Next.js calls the APIs of the contexts by name, such as `http://facility:8080`."
  type        = string
}

variable "vpc_id" {
  description = "The ID of the VPC."
  type        = string
}

variable "public_subnet_ids" {
  description = "The IDs of the public subnets, for the load balancer."
  type        = list(string)
}

variable "subnet_ids" {
  description = "The IDs of the private subnets, for the tasks."
  type        = list(string)
}

variable "services_security_group_id" {
  description = "The ID of the security group that every task of the environment joins. It lets the task call the APIs of the contexts and reach the AWS services through the endpoints of the VPC."
  type        = string
}

variable "domain_name" {
  description = "The domain name of the site, such as `stg.example.com`. The module requests a certificate for it and points the name at the load balancer."
  type        = string
}

variable "hosted_zone_id" {
  description = "The ID of the public hosted zone in Amazon Route 53 that holds `domain_name`. The records of the name and of the validation of the certificate go there."
  type        = string
}

variable "certificate_arn" {
  description = "The ARN of a certificate in AWS Certificate Manager that covers `domain_name`, in the Region of the load balancer. By default the module requests a certificate and validates it through the hosted zone; name one here to use a certificate that exists, such as a wildcard certificate of the team."
  type        = string
  default     = null
}

variable "environment_variables" {
  description = "The environment variables of the server, such as `FACILITY_API_URL = \"http://facility:8080\"`. They are not secret: the server of Next.js holds no secret."
  type        = map(string)
  default     = {}
}

variable "desired_count" {
  description = "How many tasks run."
  type        = number
  default     = 1

  validation {
    condition     = var.desired_count >= 1
    error_message = "The site needs at least one task."
  }
}

variable "cpu" {
  description = "The CPU units of a task; 1024 is one vCPU. Fargate accepts certain pairs of CPU and memory."
  type        = number
  default     = 256
}

variable "memory" {
  description = "The memory of a task, in MiB."
  type        = number
  default     = 512
}

variable "cpu_architecture" {
  description = "The CPU architecture that the image is built for: `X86_64` or `ARM64`."
  type        = string
  default     = "X86_64"

  validation {
    condition     = contains(["X86_64", "ARM64"], var.cpu_architecture)
    error_message = "Choose X86_64 or ARM64."
  }
}

variable "deletion_protection" {
  description = "Whether the load balancer refuses to be deleted."
  type        = bool
  default     = true
}

variable "log_retention_days" {
  description = "How many days CloudWatch Logs keeps the logs of the server."
  type        = number
  default     = 30
}
