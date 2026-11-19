variable "environment" {
  description = "The name of the environment, such as `stg`. The task definitions, the roles, the log group, and the security group of the context are named after it."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]*$", var.environment))
    error_message = "The name of the environment is in lowercase letters, digits, and hyphens, and starts with a letter, such as stg."
  }
}

variable "name" {
  description = "The name of the context, such as `reservation`: the name of its command in `backend/cmd/`. The service of its API has this name, and the other contexts call the API at `http://<name>:8080`."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9]*(-[a-z0-9]+)*$", var.name))
    error_message = "The name of a context is in lowercase letters, digits, and single hyphens, and starts with a letter, such as reservation."
  }

  # The longest name of the module: the role that starts the tasks. IAM
  # allows 64 characters.
  validation {
    condition     = length("${var.environment}-${var.name}-execution") <= 64
    error_message = "The names of the environment and the context together are too long: \"<environment>-<name>-execution\", the name of a role, must fit in 64 characters."
  }
}

variable "image" {
  description = "The image that every process of the context runs, with a tag or a digest, such as `<account>.dkr.ecr.<region>.amazonaws.com/reservation:<tag>`: built from `backend/Dockerfile` with the build argument CONTEXT, and pushed to a repository of the module registry."
  type        = string

  validation {
    condition     = can(regex("^[^\\s:@]+(:[0-9]+)?/[^\\s:@]+(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[0-9a-f]{64})$", var.image))
    error_message = "Name the image with its registry, its repository, and a tag or a digest, such as 123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/reservation:1.2.0."
  }
}

variable "repository_arn" {
  description = "The ARN of the Amazon ECR repository of the image: `repository_arns` of the module registry. The tasks may pull from this repository alone."
  type        = string
}

variable "cluster_arn" {
  description = "The ARN of the ECS cluster: `cluster_arn` of the module platform."
  type        = string
}

variable "namespace_arn" {
  description = "The ARN of the namespace of ECS Service Connect: `namespace_arn` of the module platform."
  type        = string
}

variable "vpc_id" {
  description = "The ID of the VPC: `vpc_id` of the module network."
  type        = string
}

variable "subnet_ids" {
  description = "The IDs of the private subnets in which the tasks run: `private_subnet_ids` of the module network."
  type        = list(string)

  validation {
    condition     = length(var.subnet_ids) > 0
    error_message = "Name at least one private subnet."
  }
}

variable "services_security_group_id" {
  description = "The ID of the security group that every task of every service joins: `services_security_group_id` of the module platform."
  type        = string
}

variable "database" {
  description = <<-EOT
    The database of the context.
    `security_group_id` is the security group of the database: the module allows PostgreSQL (5432) from the tasks of the context into it.
    `secret_arn` is the ARN of a secret of Secrets Manager whose value is JSON with the key `url`, the URL of the runtime role of the context, which the processes get as DATABASE_URL.
    `kms_key_arn` is the ARN of the customer managed key of AWS KMS that encrypts the secret, if one does; null for the key that AWS manages for Secrets Manager.
  EOT
  type = object({
    security_group_id = string
    secret_arn        = string
    kms_key_arn       = optional(string)
  })
}

variable "api" {
  description = <<-EOT
    The API of the context, or null for a context without one, such as Notification.
    The API runs as the ECS service `<name>`, from the image with no command, and the other services call it at `http://<name>:8080`.
    `cpu` and `memory` are those of one task, in CPU units and MiB, as Fargate allows them; the proxy of ECS Service Connect runs in the task too.
  EOT
  type = object({
    desired_count = optional(number, 1)
    cpu           = optional(number, 256)
    memory        = optional(number, 512)
  })
  default = null

  validation {
    condition     = var.api == null || try(var.api.desired_count >= 0 && floor(var.api.desired_count) == var.api.desired_count, false)
    error_message = "The desired count of the API is a whole number, 0 or more."
  }

  validation {
    condition = var.api == null || try(
      contains([256, 512, 1024, 2048, 4096, 8192, 16384], var.api.cpu)
      && var.api.memory >= 2 * var.api.cpu && var.api.memory <= 8 * var.api.cpu,
      false,
    )
    error_message = "Fargate runs a task with 256, 512, 1024, 2048, 4096, 8192, or 16384 CPU units, and with two to eight MiB of memory for each CPU unit, such as cpu = 256 and memory = 512."
  }
}

variable "workers" {
  description = <<-EOT
    The workers of the context, by name, such as `relay`: processes that serve no requests (TPL-0014). Each runs as the ECS service `<name>-<key>`, from the image of the context.
    `command` is the arguments of the image, such as `["relay"]`; an empty list runs the image as it is, as the worker of Notification does.
    `cpu` and `memory` are those of one task, in CPU units and MiB, as Fargate allows them; the proxy of ECS Service Connect runs in the task too.
  EOT
  type = map(object({
    command       = list(string)
    desired_count = optional(number, 1)
    cpu           = optional(number, 256)
    memory        = optional(number, 512)
  }))
  default = {}

  validation {
    condition     = var.api != null || length(var.workers) > 0
    error_message = "A context runs an API, a worker, or both."
  }

  validation {
    condition     = alltrue([for key in keys(var.workers) : can(regex("^[a-z][a-z0-9]*(-[a-z0-9]+)*$", key))])
    error_message = "The name of a worker is in lowercase letters, digits, and single hyphens, and starts with a letter, such as relay."
  }

  validation {
    condition     = alltrue([for worker in values(var.workers) : worker.desired_count >= 0 && floor(worker.desired_count) == worker.desired_count])
    error_message = "The desired count of a worker is a whole number, 0 or more."
  }

  validation {
    condition = alltrue([
      for worker in values(var.workers) :
      contains([256, 512, 1024, 2048, 4096, 8192, 16384], worker.cpu)
      && worker.memory >= 2 * worker.cpu && worker.memory <= 8 * worker.cpu
    ])
    error_message = "Fargate runs a task with 256, 512, 1024, 2048, 4096, 8192, or 16384 CPU units, and with two to eight MiB of memory for each CPU unit, such as cpu = 256 and memory = 512."
  }
}

variable "environment_variables" {
  description = "Environment variables of every process of the context, such as `{ FACILITY_API_URL = \"http://facility:8080\" }`. The module sets AWS_REGION, DATABASE_URL (from the secret), EVENT_BUS_NAME (when the context publishes events), and EVENTS_QUEUE_URL (with a subscription) itself. A secret belongs in Secrets Manager, not here: the values are plain in the task definition."
  type        = map(string)
  default     = {}

  validation {
    condition     = length(setintersection(keys(var.environment_variables), ["AWS_REGION", "DATABASE_URL", "EVENT_BUS_NAME", "EVENTS_QUEUE_URL"])) == 0
    error_message = "The module sets AWS_REGION, DATABASE_URL, EVENT_BUS_NAME, and EVENTS_QUEUE_URL itself; DATABASE_URL comes from the secret of the database."
  }
}

variable "event_bus" {
  description = "The event bus of the environment: `event_bus` of the module platform."
  type = object({
    name = string
    arn  = string
  })
}

variable "publishes_events" {
  description = "Whether the context publishes integration events (TPL-0013). When true, the processes get EVENT_BUS_NAME, and the role of the tasks may put events on the bus and describe it: the relay checks at its start that the bus exists."
  type        = bool
  default     = false
}

variable "subscription" {
  description = <<-EOT
    The events that the context consumes, or null for a context that consumes none (TPL-0013).
    `sources` are the CloudEvents `source` of each publishing context, such as `/contexts/reservation`, and `event_types` the CloudEvents `type` of each event, such as `reservation.ReservationConfirmed.v1`.
    The module creates the queue of the context with the module event-subscription, gives the processes its URL as EVENTS_QUEUE_URL, and lets the role of the tasks receive and delete its messages.
  EOT
  type = object({
    sources     = list(string)
    event_types = list(string)
  })
  default = null

  validation {
    condition     = var.subscription == null || try(length(var.subscription.sources) > 0 && length(var.subscription.event_types) > 0, false)
    error_message = "A subscription names at least one source and one type of event."
  }
}

variable "iam_statements" {
  description = "More permissions of the role of the tasks, beyond the events and the queue, such as `[{ actions = [\"ses:SendEmail\"], resources = [\"<ARN of the identity>\"] }]` for a context that sends mail through Amazon SES."
  type = list(object({
    actions   = list(string)
    resources = list(string)
  }))
  default = []

  validation {
    condition     = alltrue([for statement in var.iam_statements : length(statement.actions) > 0 && length(statement.resources) > 0])
    error_message = "A statement names at least one action and one resource."
  }
}

variable "cpu_architecture" {
  description = "The CPU architecture of the image: `X86_64` or `ARM64`. Fargate on ARM64 (Graviton) costs less for the same size of task."
  type        = string
  default     = "X86_64"

  validation {
    condition     = contains(["X86_64", "ARM64"], var.cpu_architecture)
    error_message = "Choose X86_64 or ARM64."
  }
}

variable "log_retention_days" {
  description = "How many days CloudWatch Logs keeps the logs of the context. 0 keeps them for ever."
  type        = number
  default     = 30

  validation {
    condition     = contains([0, 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653], var.log_retention_days)
    error_message = "CloudWatch Logs accepts 0 or one of 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, and 3653 days."
  }
}
