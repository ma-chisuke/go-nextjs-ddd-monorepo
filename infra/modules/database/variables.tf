variable "environment" {
  description = "The name of the environment, such as `stg`. Every resource is named after it."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,30}[a-z0-9]$", var.environment))
    error_message = "The name of the environment must be 2 to 32 lowercase letters, digits, and hyphens, start with a letter, and not end with a hyphen: it names the cluster, whose identifier allows nothing else."
  }
}

# Network ---------------------------------------------------------------------

variable "vpc_id" {
  description = "The ID of the VPC (`vpc_id` of the module network)."
  type        = string
}

variable "subnet_ids" {
  description = "The IDs of the database subnets, where the cluster runs (`database_subnet_ids` of the module network). Aurora needs subnets in two Availability Zones at least."
  type        = list(string)

  validation {
    condition     = length(var.subnet_ids) >= 2
    error_message = "Name two database subnets at least, in two Availability Zones."
  }
}

variable "task_subnet_ids" {
  description = "The IDs of the private subnets, where the task that prepares the database runs (`private_subnet_ids` of the module network)."
  type        = list(string)

  validation {
    condition     = length(var.task_subnet_ids) >= 1
    error_message = "Name one private subnet at least."
  }
}

variable "endpoints_security_group_id" {
  description = "The ID of the security group of the interface endpoints (`endpoints_security_group_id` of the module network). The task reaches Amazon ECR, CloudWatch Logs, and Secrets Manager through them."
  type        = string
}

variable "s3_prefix_list_id" {
  description = "The ID of the prefix list of Amazon S3 (`s3_prefix_list_id` of the module network). The task pulls the layers of its image from there."
  type        = string
}

# Cluster ---------------------------------------------------------------------

variable "engine_version" {
  description = "The version of Aurora PostgreSQL, such as `18.6`. A version below 18 does not run the schemas of the template: they call `uuidv7()`, which PostgreSQL 18 brought (TPL-0009)."
  type        = string
  default     = "18.6"
}

variable "database_name" {
  description = "The name of the database that every context shares (TPL-0009)."
  type        = string
  default     = "app"

  validation {
    condition     = can(regex("^[a-z][a-z0-9_]{0,62}$", var.database_name))
    error_message = "The name of the database must be up to 63 lowercase letters, digits, and underscores, and start with a letter."
  }
}

variable "capacity" {
  description = "The capacity of the instances, exactly one of two kinds (TPL-0016). `serverless = { min_acu, max_acu, seconds_until_auto_pause }` is Aurora Serverless v2, between `min_acu` and `max_acu` Aurora capacity units; with `min_acu = 0` the cluster pauses after `seconds_until_auto_pause` seconds without a connection (300 to 86400, by default 300), and resumes on the next connection. `provisioned = { instance_class }` is instances of a fixed class, such as `db.r8g.large`. The default is Serverless v2 from 0 to 2 ACU, which costs nothing but storage while it pauses."
  type = object({
    serverless = optional(object({
      min_acu                  = number
      max_acu                  = number
      seconds_until_auto_pause = optional(number, 300)
    }))
    provisioned = optional(object({
      instance_class = string
    }))
  })
  default = {
    serverless = {
      min_acu = 0
      max_acu = 2
    }
  }

  validation {
    condition     = (var.capacity.serverless == null) != (var.capacity.provisioned == null)
    error_message = "Choose exactly one kind of capacity: serverless or provisioned."
  }

  validation {
    condition = var.capacity.serverless == null ? true : (
      var.capacity.serverless.min_acu >= 0 && var.capacity.serverless.max_acu >= 1 && var.capacity.serverless.max_acu <= 256
      && var.capacity.serverless.min_acu * 2 == floor(var.capacity.serverless.min_acu * 2)
      && var.capacity.serverless.max_acu * 2 == floor(var.capacity.serverless.max_acu * 2)
    )
    error_message = "Serverless v2 counts capacity in steps of 0.5 ACU: the minimum from 0, the maximum from 1 to 256."
  }

  validation {
    condition     = var.capacity.serverless == null ? true : var.capacity.serverless.min_acu <= var.capacity.serverless.max_acu
    error_message = "The minimum capacity must not exceed the maximum."
  }

  validation {
    condition = var.capacity.serverless == null ? true : (
      var.capacity.serverless.seconds_until_auto_pause >= 300 && var.capacity.serverless.seconds_until_auto_pause <= 86400
    )
    error_message = "Aurora pauses after 300 to 86400 seconds without a connection."
  }

  # db.serverless has one dot, so it does not pass either.
  validation {
    condition     = var.capacity.provisioned == null ? true : can(regex("^db\\.[a-z0-9]+\\.[a-z0-9]+$", var.capacity.provisioned.instance_class))
    error_message = "Name an instance class such as db.r8g.large; Serverless v2 is chosen with serverless, not with db.serverless."
  }
}

variable "instance_count" {
  description = "How many instances the cluster has: the writer, and readers after it. A reader takes over when the writer fails."
  type        = number
  default     = 1

  validation {
    condition     = var.instance_count >= 1 && var.instance_count <= 16 && floor(var.instance_count) == var.instance_count
    error_message = "A cluster has 1 to 16 instances."
  }
}

variable "deletion_protection" {
  description = "Whether AWS refuses to delete the cluster. Turn it off, and apply, before you destroy the cluster on purpose."
  type        = bool
  default     = true
}

variable "backup_retention_days" {
  description = "How many days Aurora keeps the automated backups, from which the cluster can be restored to any second."
  type        = number
  default     = 7

  validation {
    condition     = var.backup_retention_days >= 1 && var.backup_retention_days <= 35 && floor(var.backup_retention_days) == var.backup_retention_days
    error_message = "Aurora keeps automated backups for 1 to 35 days."
  }
}

# Roles of the contexts -------------------------------------------------------

variable "contexts" {
  description = "The bounded contexts, such as `[\"facility\", \"reservation\", \"notification\"]`: the directories of `backend/internal/` with a `schema.sql`. Each one gets two secrets, for its roles `<context>_owner` and `<context>_runtime` (TPL-0009)."
  type        = set(string)

  validation {
    condition     = length(var.contexts) >= 1 && alltrue([for context in var.contexts : can(regex("^[a-z][a-z0-9_]{0,50}$", context))])
    error_message = "Name one context at least, each in lowercase letters, digits, and underscores, as its directory in backend/internal/."
  }
}

variable "password_version" {
  description = "The version of the passwords of the roles of the contexts. Raising it writes a new password to each secret; the task that prepares the database then sets them on the roles, and the services read them when they start again."
  type        = number
  default     = 1

  validation {
    condition     = var.password_version >= 1 && floor(var.password_version) == var.password_version
    error_message = "The version is a whole number from 1."
  }
}

# The task that prepares the database ----------------------------------------

variable "db_image" {
  description = "The image of `db/Dockerfile` in Amazon ECR, best with its digest: `<account>.dkr.ecr.<region>.amazonaws.com/<repository>@sha256:<digest>`. The task that prepares the database runs it."
  type        = string

  validation {
    condition     = can(regex(local.ecr_image_pattern, var.db_image))
    error_message = "The image must be in Amazon ECR, such as 123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/db@sha256:<digest>: the task reaches no other registry."
  }
}

variable "task_cpu_architecture" {
  description = "The CPU architecture of the task that prepares the database: `X86_64` or `ARM64`. The image must be built for it; the deployment builds every image for `linux/amd64` (`tools/scripts/images.sh`), which is the default here and in the modules of the services."
  type        = string
  default     = "X86_64"

  validation {
    condition     = contains(["ARM64", "X86_64"], var.task_cpu_architecture)
    error_message = "Choose ARM64 or X86_64."
  }
}

variable "log_retention_days" {
  description = "How many days CloudWatch Logs keeps what the task that prepares the database printed."
  type        = number
  default     = 30
}
