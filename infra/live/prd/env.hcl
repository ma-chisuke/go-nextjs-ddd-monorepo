# What prd is: an AWS account of its own, sized for its load and spread over
# the zones (TPL-0015, TPL-0016). stg/env.hcl has the same keys with other
# values.

locals {
  name = "prd"

  # Replace the placeholders with your account, domain name, and hosted zone
  # before the first apply (docs/guides/deploy.md).
  account_id     = "222222222222"
  region         = "ap-northeast-1"
  domain_name    = "example.com"
  hosted_zone_id = "Z0000000000000PRD000"

  availability_zones = ["ap-northeast-1a", "ap-northeast-1c", "ap-northeast-1d"]

  # An interface of each endpoint in every zone, so the loss of a zone does
  # not cut the tasks of the other zones off from AWS.
  interface_endpoints_in_every_zone = true

  # Provisioned instances for a steady load: a writer and a reader in
  # another zone. Aurora Serverless v2 with a minimum above zero is the
  # other choice; the module takes either (infra/modules/database).
  database_capacity = {
    provisioned = {
      instance_class = "db.r8g.large"
    }
  }
  database_instance_count = 2

  # Whether the database and the load balancer refuse to be deleted.
  deletion_protection = true

  # How many tasks each service runs.
  desired_count = 2

  # The address that the mail of Notification comes from, and the identity
  # that covers it: a domain that is verified in Amazon SES in this account.
  mail_from     = "Meeting rooms <rooms@example.com>"
  mail_identity = "example.com"
}
