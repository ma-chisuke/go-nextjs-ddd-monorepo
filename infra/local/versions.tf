terraform {
  required_version = ">= 1.17"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.73.0"
    }
  }

  # The state describes an emulator that forgets everything when it stops, so
  # it stays on this machine: tools/scripts/aws.sh keeps it outside the
  # repository, and throws it away when the emulator restarted.
  backend "local" {}
}
