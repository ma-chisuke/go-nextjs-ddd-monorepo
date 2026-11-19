# What the module plans, without an AWS account: the AWS provider is a mock,
# so `terraform test` needs no credentials and creates nothing. The runs with
# `command = apply` apply to the mock, so that the values that AWS would
# compute, such as ARNs, are known to the assertions. The passwords come from
# the real random provider, which runs on this machine.

mock_provider "aws" {
  mock_data "aws_region" {
    defaults = {
      region = "ap-northeast-1"
    }
  }

  mock_data "aws_partition" {
    defaults = {
      partition = "aws"
    }
  }

  mock_data "aws_iam_policy_document" {
    defaults = {
      json = "{}"
    }
  }

  # The provider checks that these are ARNs where other resources use them.
  mock_resource "aws_kms_key" {
    defaults = {
      arn = "arn:aws:kms:ap-northeast-1:123456789012:key/00000000-0000-0000-0000-000000000000"
    }
  }

  mock_resource "aws_iam_role" {
    defaults = {
      arn = "arn:aws:iam::123456789012:role/test"
    }
  }

  # RDS returns the secret of the master user once it has made it.
  mock_resource "aws_rds_cluster" {
    defaults = {
      master_user_secret = [{
        secret_arn    = "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:rds!cluster-test"
        kms_key_id    = ""
        secret_status = "active"
      }]
    }
  }
}

variables {
  environment                 = "test"
  vpc_id                      = "vpc-0123456789abcdef0"
  subnet_ids                  = ["subnet-database-a", "subnet-database-c"]
  task_subnet_ids             = ["subnet-private-a", "subnet-private-c"]
  endpoints_security_group_id = "sg-endpoints"
  s3_prefix_list_id           = "pl-s3"
  contexts                    = ["facility", "reservation"]
  db_image                    = "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/db@sha256:0000000000000000000000000000000000000000000000000000000000000000"
}

run "serverless_v2_from_zero_by_default" {
  command = plan

  assert {
    condition     = aws_rds_cluster.this.engine == "aurora-postgresql" && aws_rds_cluster.this.engine_mode == "provisioned" && aws_rds_cluster.this.engine_version == "18.6"
    error_message = "Serverless v2 is a provisioned cluster of Aurora PostgreSQL 18."
  }

  assert {
    condition = (
      length(aws_rds_cluster.this.serverlessv2_scaling_configuration) == 1
      && aws_rds_cluster.this.serverlessv2_scaling_configuration[0].min_capacity == 0
      && aws_rds_cluster.this.serverlessv2_scaling_configuration[0].max_capacity == 2
      && aws_rds_cluster.this.serverlessv2_scaling_configuration[0].seconds_until_auto_pause == 300
    )
    error_message = "By default the cluster scales from 0 to 2 ACU, and pauses after 300 seconds without a connection."
  }

  assert {
    condition     = length(aws_rds_cluster_instance.this) == 1 && aws_rds_cluster_instance.this[0].instance_class == "db.serverless"
    error_message = "By default the cluster has one instance of Serverless v2."
  }

  assert {
    condition     = alltrue([for instance in aws_rds_cluster_instance.this : instance.publicly_accessible == false])
    error_message = "No instance may have a public address."
  }
}

run "no_pause_above_zero" {
  command = plan

  variables {
    capacity = {
      serverless = {
        min_acu = 0.5
        max_acu = 4
      }
    }
  }

  assert {
    condition     = aws_rds_cluster.this.serverlessv2_scaling_configuration[0].min_capacity == 0.5 && local.seconds_until_auto_pause == null
    error_message = "A cluster with a minimum above 0 ACU does not pause, so it gets no time to pause."
  }
}

run "provisioned_instances_on_request" {
  command = plan

  variables {
    capacity = {
      provisioned = {
        instance_class = "db.r8g.large"
      }
    }
    instance_count = 2
  }

  assert {
    condition     = length(aws_rds_cluster.this.serverlessv2_scaling_configuration) == 0
    error_message = "A provisioned cluster has no scaling of Serverless v2."
  }

  assert {
    condition     = length(aws_rds_cluster_instance.this) == 2 && alltrue([for instance in aws_rds_cluster_instance.this : instance.instance_class == "db.r8g.large"])
    error_message = "Each instance has the class given."
  }
}

run "both_kinds_of_capacity_are_rejected" {
  command = plan

  variables {
    capacity = {
      serverless = {
        min_acu = 0
        max_acu = 2
      }
      provisioned = {
        instance_class = "db.r8g.large"
      }
    }
  }

  expect_failures = [var.capacity]
}

run "no_kind_of_capacity_is_rejected" {
  command = plan

  variables {
    capacity = {}
  }

  expect_failures = [var.capacity]
}

run "a_minimum_above_the_maximum_is_rejected" {
  command = plan

  variables {
    capacity = {
      serverless = {
        min_acu = 4
        max_acu = 2
      }
    }
  }

  expect_failures = [var.capacity]
}

run "a_pause_shorter_than_five_minutes_is_rejected" {
  command = plan

  variables {
    capacity = {
      serverless = {
        min_acu                  = 0
        max_acu                  = 2
        seconds_until_auto_pause = 299
      }
    }
  }

  expect_failures = [var.capacity]
}

run "a_pause_longer_than_a_day_is_rejected" {
  command = plan

  variables {
    capacity = {
      serverless = {
        min_acu                  = 0
        max_acu                  = 2
        seconds_until_auto_pause = 86401
      }
    }
  }

  expect_failures = [var.capacity]
}

run "a_capacity_off_the_half_steps_is_rejected" {
  command = plan

  variables {
    capacity = {
      serverless = {
        min_acu = 0
        max_acu = 2.25
      }
    }
  }

  expect_failures = [var.capacity]
}

run "db_serverless_as_a_provisioned_class_is_rejected" {
  command = plan

  variables {
    capacity = {
      provisioned = {
        instance_class = "db.serverless"
      }
    }
  }

  expect_failures = [var.capacity]
}

run "an_image_outside_amazon_ecr_is_rejected" {
  command = plan

  variables {
    db_image = "docker.io/library/postgres:18.7"
  }

  expect_failures = [var.db_image]
}

run "the_cluster_is_encrypted_and_protected_from_deletion" {
  command = apply

  assert {
    condition     = aws_rds_cluster.this.storage_encrypted && aws_rds_cluster.this.kms_key_id == aws_kms_key.this.arn && aws_kms_key.this.enable_key_rotation
    error_message = "The cluster is encrypted at rest with a key of the account, which rotates."
  }

  assert {
    condition     = aws_rds_cluster.this.deletion_protection && aws_rds_cluster.this.backup_retention_period == 7 && aws_rds_cluster.this.final_snapshot_identifier == "test-final"
    error_message = "By default the cluster is protected from deletion, keeps backups for 7 days, and leaves a snapshot when it is destroyed."
  }

  assert {
    condition     = aws_rds_cluster.this.manage_master_user_password && aws_rds_cluster.this.iam_database_authentication_enabled != true
    error_message = "RDS keeps the password of the master user; the roles sign in with passwords, not with IAM (TPL-0014)."
  }
}

run "each_context_gets_a_secret_for_each_role" {
  command = apply

  assert {
    condition = toset([for secret in aws_secretsmanager_secret.role : secret.name]) == toset([
      "test/database/facility/owner",
      "test/database/facility/runtime",
      "test/database/reservation/owner",
      "test/database/reservation/runtime",
    ])
    error_message = "Each context needs a secret for its owner role and one for its runtime role."
  }

  # The provider stores an empty secret_string when the value went through
  # the write-only argument.
  assert {
    condition = alltrue([
      for version in aws_secretsmanager_secret_version.role :
      version.secret_string_wo_version == 1 && (version.secret_string == null || version.secret_string == "")
    ])
    error_message = "The passwords go to Secrets Manager through the write-only argument, at version 1 by default, and never into the state."
  }

  assert {
    condition     = toset(keys(output.runtime_secret_arns)) == toset(["facility", "reservation"]) && output.runtime_secret_arns["facility"] == aws_secretsmanager_secret.role["facility/runtime"].arn
    error_message = "The output runtime_secret_arns names the secret of the runtime role of each context."
  }
}

run "the_task_reaches_no_address_range" {
  command = plan

  assert {
    condition = alltrue([
      for rule in [aws_vpc_security_group_egress_rule.task_postgres, aws_vpc_security_group_egress_rule.task_endpoints, aws_vpc_security_group_egress_rule.task_s3] :
      rule.cidr_ipv4 == null && rule.cidr_ipv6 == null
    ])
    error_message = "No rule of the security group of the task may name an address range: the task reaches the cluster and the AWS services by their groups and prefix lists."
  }

  assert {
    condition     = aws_vpc_security_group_egress_rule.task_endpoints.referenced_security_group_id == "sg-endpoints" && aws_vpc_security_group_egress_rule.task_s3.prefix_list_id == "pl-s3"
    error_message = "The task reaches the endpoints of the network, and Amazon S3 by its prefix list."
  }

  assert {
    condition     = aws_vpc_security_group_egress_rule.task_postgres.from_port == 5432 && aws_vpc_security_group_ingress_rule.cluster_from_task.from_port == 5432 && aws_vpc_security_group_ingress_rule.cluster_from_task.cidr_ipv4 == null
    error_message = "The cluster accepts PostgreSQL from the task by its group, and no address range."
  }
}

run "the_task_gets_the_secrets_of_every_role" {
  command = apply

  assert {
    condition = (
      aws_ecs_task_definition.db.network_mode == "awsvpc"
      && aws_ecs_task_definition.db.requires_compatibilities == toset(["FARGATE"])
      && jsondecode(aws_ecs_task_definition.db.container_definitions)[0].readonlyRootFilesystem
    )
    error_message = "The task runs on Fargate in the network of the VPC, with a read-only root file system."
  }

  assert {
    condition = toset([for secret in local.container.secrets : secret.name]) == toset([
      "PGUSER", "PGPASSWORD",
      "DB_OWNER_PASSWORD_FACILITY", "DB_RUNTIME_PASSWORD_FACILITY",
      "DB_OWNER_PASSWORD_RESERVATION", "DB_RUNTIME_PASSWORD_RESERVATION",
    ])
    error_message = "The task needs the master user, and the password of each role of each context, as db/prepare.sh reads them."
  }

  assert {
    condition = alltrue([
      for secret in local.container.secrets :
      endswith(secret.valueFrom, secret.name == "PGUSER" ? ":username::" : ":password::")
    ])
    error_message = "Each variable is one key of a JSON secret."
  }

  assert {
    condition     = contains([for variable in local.container.environment : "${variable.name}=${variable.value}"], "PGSSLMODE=require")
    error_message = "The task connects with TLS."
  }

  assert {
    condition = toset(one([for statement in data.aws_iam_policy_document.execution.statement : statement.resources if statement.sid == "ReadTheSecrets"])) == toset(concat(
      [aws_rds_cluster.this.master_user_secret[0].secret_arn],
      [for secret in aws_secretsmanager_secret.role : secret.arn],
    ))
    error_message = "The execution role reads exactly the secrets of the task."
  }

  assert {
    condition     = one([for statement in data.aws_iam_policy_document.execution.statement : statement.resources if statement.sid == "PullTheImage"]) == toset(["arn:aws:ecr:ap-northeast-1:123456789012:repository/db"])
    error_message = "The execution role pulls from the repository of the image only."
  }

  assert {
    condition     = output.db_task.subnet_ids == tolist(["subnet-private-a", "subnet-private-c"]) && output.db_task.container_name == "db" && output.db_task.security_group_id == aws_security_group.task.id
    error_message = "The output db_task has what aws ecs run-task needs."
  }
}
