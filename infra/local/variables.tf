variable "endpoint" {
  description = "The URL of AWS Emulator on this machine. tools/scripts/aws.sh sets it."
  type        = string

  validation {
    condition     = can(regex("^https://(localhost|127\\.0\\.0\\.1):[0-9]+$", var.endpoint))
    error_message = "The endpoint must be AWS Emulator on this machine, such as https://localhost:19297. This configuration is never applied to AWS."
  }
}

variable "ca_bundle" {
  description = "The path of the CA that signs the certificate of the endpoint. tools/scripts/aws.sh copies it from the emulator and sets this."
  type        = string
}
