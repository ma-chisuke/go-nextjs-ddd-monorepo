# What stg is: an AWS account of its own, small, and almost free while idle
# (TPL-0015, TPL-0016). prd/env.hcl has the same keys with other values.

locals {
  name = "stg"

  # Replace the placeholders with your account, domain name, and hosted zone
  # before the first apply (docs/guides/deploy.md).
  account_id     = "111111111111"
  region         = "ap-northeast-1"
  domain_name    = "stg.example.com"
  hosted_zone_id = "Z0000000000000STG000"

  availability_zones = ["ap-northeast-1a", "ap-northeast-1c"]

  # One network interface for each endpoint: cheaper, and gone with its zone.
  interface_endpoints_in_every_zone = false

  # Aurora Serverless v2 that pauses when nobody is connected.
  database_capacity = {
    serverless = {
      min_acu = 0
      max_acu = 2
    }
  }
  database_instance_count = 1

  # Whether the database and the load balancer refuse to be deleted.
  deletion_protection = false

  # How many tasks each service runs.
  desired_count = 1

  # The address that the mail of Notification comes from, and the identity
  # that covers it: a domain that is verified in Amazon SES in this account.
  mail_from     = "Meeting rooms <rooms@stg.example.com>"
  mail_identity = "stg.example.com"
}
