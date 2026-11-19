# The providers of every unit, at exact versions, in the Terraform language.
# root.hcl copies this file into each unit as versions_override.tf, next to
# the lock file providers.lock.hcl, so all units install the same providers
# with the same checksums. After a change here, run `make infra-lock`.
terraform {
  required_version = ">= 1.17"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.73.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.9.1"
    }
  }
}
