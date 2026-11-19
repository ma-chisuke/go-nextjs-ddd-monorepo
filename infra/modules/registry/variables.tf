variable "repositories" {
  description = "The names of the repositories, one for each image: each context, the web front end, and any other image that the environments run, such as `[\"facility\", \"reservation\", \"notification\", \"web\"]`. A name is in lowercase letters, digits, and hyphens, and starts with a letter."
  type        = set(string)

  validation {
    condition     = length(var.repositories) > 0
    error_message = "Name at least one repository."
  }

  validation {
    condition     = alltrue([for name in var.repositories : can(regex("^[a-z][a-z0-9]*(-[a-z0-9]+)*$", name))])
    error_message = "A repository name is in lowercase letters, digits, and single hyphens, and starts with a letter, such as facility."
  }
}

variable "kept_images" {
  description = "How many images each repository keeps. When a push makes one more, the lifecycle policy expires the oldest. Keep enough for a rollback: an image that a task definition still names can expire, and its service then fails to start a task."
  type        = number
  default     = 30

  validation {
    condition     = var.kept_images >= 1 && floor(var.kept_images) == var.kept_images
    error_message = "Keep at least one image, as a whole number."
  }
}
