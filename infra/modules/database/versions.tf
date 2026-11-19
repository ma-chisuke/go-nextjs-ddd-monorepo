terraform {
  required_version = ">= 1.17"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 6.0"
    }
    # The ephemeral resource random_password, whose value never enters the
    # state.
    random = {
      source  = "hashicorp/random"
      version = ">= 3.7"
    }
  }
}
