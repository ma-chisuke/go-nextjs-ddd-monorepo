variable "environment" {
  description = "The name of the environment, such as `stg`. It is also the name of the GitHub Environment whose jobs may assume the role that deploys."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.environment))
    error_message = "The name of the environment holds lowercase letters, digits, and hyphens only."
  }
}

variable "repository" {
  description = "The repository whose workflows may assume the roles, written as GitHub writes it in the subject of its tokens. For a repository created before July 15, 2026 that is `owner/name`. A repository created later, renamed, or transferred uses the immutable form with both IDs, `owner@owner-id/name@repository-id`; `gh api repos/OWNER/NAME --jq '\"\\(.owner.login)@\\(.owner.id)/\\(.name)@\\(.id)\"'` prints it."
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+(@[0-9]+)?/[A-Za-z0-9_.-]+(@[0-9]+)?$", var.repository))
    error_message = "Name one repository, as owner/name or owner@owner-id/name@repository-id, without a wildcard."
  }
}

variable "state_bucket" {
  description = "The name of the S3 bucket that holds the Terraform state of this environment. The role that plans may write and delete the lock files in it, and nothing else."
  type        = string
}

variable "create_provider" {
  description = "Whether to create the OIDC identity provider for GitHub in this account. An account holds one provider for each URL: set this to `false` when the account already has it."
  type        = bool
  default     = true
}

variable "deploy_policy_arns" {
  description = "The policies of the role that deploys. The default is `AdministratorAccess`: a role that applies this infrastructure creates IAM roles, so a narrower policy would not limit what it can reach. What limits the role is who may assume it: the jobs of one GitHub Environment of one repository."
  type        = list(string)
  default     = ["arn:aws:iam::aws:policy/AdministratorAccess"]

  validation {
    condition     = length(var.deploy_policy_arns) > 0
    error_message = "The role that deploys needs at least one policy."
  }
}
