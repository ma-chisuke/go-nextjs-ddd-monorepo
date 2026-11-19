# The database of one environment (TPL-0009, TPL-0016): an Aurora PostgreSQL
# cluster with one database that every bounded context shares, the secrets of
# the roles of each context, and the ECS task that prepares the database at
# deploy time.
#
# The capacity is Aurora Serverless v2 by default, from 0 ACU, so an idle
# environment pauses and pays for storage only; var.capacity chooses
# provisioned instances instead. A cluster pauses only while no connection is
# open, so what connects to it follows the rules of TPL-0016:
#   - the connection pool of a service keeps no idle connection
#     (backend/internal/platform/postgres/pool.go);
#   - the health check of a service does not touch the database;
#   - no RDS Proxy, which keeps connections open, and no logical replication;
#   - resuming takes about 15 seconds, 30 or more after a long pause, so a
#     connection waits that long, and tries again.
#
# The services sign in with passwords that ECS reads from Secrets Manager into
# their environment (TPL-0014); no IAM database authentication. The task of
# db/Dockerfile runs before the services: it runs db/bootstrap.sql as the
# master user, sets the password of each role from its secret, and applies the
# schema.sql of each context (TPL-0010).

locals {
  port = 5432

  serverless     = var.capacity.serverless != null
  instance_class = local.serverless ? "db.serverless" : var.capacity.provisioned.instance_class
  # Aurora pauses only from 0 ACU, so the time to pause goes with a minimum
  # of 0.
  seconds_until_auto_pause = local.serverless ? (var.capacity.serverless.min_acu == 0 ? var.capacity.serverless.seconds_until_auto_pause : null) : null

  # The two roles of each context (TPL-0009), keyed "<context>/<kind>".
  roles = merge([
    for context in var.contexts : {
      for kind in ["owner", "runtime"] : "${context}/${kind}" => {
        context = context
        kind    = kind
        name    = "${context}_${kind}"
      }
    }
  ]...)

  # An image in Amazon ECR: account, Region, repository, then a tag, a digest,
  # or both. The execution role may pull from that repository only.
  ecr_image_pattern = "^([0-9]{12})\\.dkr\\.ecr\\.([a-z0-9-]+)\\.amazonaws\\.com(?:\\.cn)?/([a-z0-9][a-z0-9._/-]*)(?::[A-Za-z0-9_][A-Za-z0-9_.-]{0,127})?(?:@sha256:[0-9a-f]{64})?$"
  image             = regex(local.ecr_image_pattern, var.db_image)
  repository_arn    = "arn:${data.aws_partition.current.partition}:ecr:${local.image[1]}:${local.image[0]}:repository/${local.image[2]}"

  # The secret in which RDS keeps the name and the password of the master user.
  master_secret_arn = aws_rds_cluster.this.master_user_secret[0].secret_arn

  container_name = "db"
}

data "aws_region" "current" {}

data "aws_partition" "current" {}

# Encryption ------------------------------------------------------------------

# The storage, the backups, and the snapshots of the cluster are encrypted
# with a key of the account, whose use CloudTrail records and whose policy the
# team controls.
resource "aws_kms_key" "this" {
  description             = "Encrypts the Aurora cluster of ${var.environment} at rest."
  enable_key_rotation     = true
  deletion_window_in_days = 30
}

resource "aws_kms_alias" "this" {
  name          = "alias/${var.environment}-database"
  target_key_id = aws_kms_key.this.key_id
}

# Cluster ---------------------------------------------------------------------

resource "aws_db_subnet_group" "this" {
  name        = "${var.environment}-database"
  description = "The database subnets of ${var.environment}."
  subnet_ids  = var.subnet_ids

  tags = {
    Name = "${var.environment}-database"
  }
}

# PostgreSQL from the security groups that a rule names, and nothing else. A
# context adds the rule for its own tasks, with this group as the target
# (output security_group_id); the task that prepares the database has its
# rule below. The cluster opens no connection, so the group has no egress
# rule.
resource "aws_security_group" "cluster" {
  name        = "${var.environment}-database"
  description = "The Aurora cluster: PostgreSQL from the groups that a rule names."
  vpc_id      = var.vpc_id

  tags = {
    Name = "${var.environment}-database"
  }
}

resource "aws_rds_cluster" "this" {
  cluster_identifier = var.environment
  engine             = "aurora-postgresql"
  # Serverless v2 is an instance class of a provisioned cluster, not an
  # engine mode of its own.
  engine_mode    = "provisioned"
  engine_version = var.engine_version
  database_name  = var.database_name
  port           = local.port

  # RDS creates the password of the master user, keeps it in a secret of
  # Secrets Manager, and rotates it. The master user owns the database and
  # may create roles; only the task that prepares the database signs in as
  # it.
  master_username             = "postgres"
  manage_master_user_password = true

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.cluster.id]

  storage_encrypted = true
  kms_key_id        = aws_kms_key.this.arn

  deletion_protection       = var.deletion_protection
  backup_retention_period   = var.backup_retention_days
  copy_tags_to_snapshot     = true
  final_snapshot_identifier = "${var.environment}-final"

  dynamic "serverlessv2_scaling_configuration" {
    for_each = local.serverless ? [var.capacity.serverless] : []
    content {
      min_capacity             = serverlessv2_scaling_configuration.value.min_acu
      max_capacity             = serverlessv2_scaling_configuration.value.max_acu
      seconds_until_auto_pause = local.seconds_until_auto_pause
    }
  }

  tags = {
    Name = var.environment
  }
}

# The first instance is the writer; the others are readers, and one of them
# takes over when the writer fails.
resource "aws_rds_cluster_instance" "this" {
  count = var.instance_count

  identifier           = "${var.environment}-${count.index + 1}"
  cluster_identifier   = aws_rds_cluster.this.id
  instance_class       = local.instance_class
  engine               = aws_rds_cluster.this.engine
  engine_version       = aws_rds_cluster.this.engine_version
  db_subnet_group_name = aws_db_subnet_group.this.name
  publicly_accessible  = false

  tags = {
    Name = "${var.environment}-${count.index + 1}"
  }
}

# Secrets of the roles --------------------------------------------------------

# One secret for each role of each context, as JSON with the keys username,
# password, and url. The service of a context reads the url of its runtime
# role as DATABASE_URL; the task that prepares the database reads the
# password of each role and sets it on the role.
resource "aws_secretsmanager_secret" "role" {
  for_each = local.roles

  name        = "${var.environment}/database/${each.value.context}/${each.value.kind}"
  description = "The role ${each.value.name} in the database ${var.database_name} of ${var.environment}: JSON with username, password, and url."
}

# A password that never enters the state of Terraform: the ephemeral resource
# makes it at each run, and the write-only argument sends it to Secrets
# Manager only when password_version changes. Letters and digits only, so it
# can stand in a URL as it is.
ephemeral "random_password" "role" {
  for_each = local.roles

  length  = 32
  special = false
}

resource "aws_secretsmanager_secret_version" "role" {
  for_each = local.roles

  secret_id = aws_secretsmanager_secret.role[each.key].id
  secret_string_wo = jsonencode({
    username = each.value.name
    password = ephemeral.random_password.role[each.key].result
    url      = "postgres://${each.value.name}:${ephemeral.random_password.role[each.key].result}@${aws_rds_cluster.this.endpoint}:${local.port}/${var.database_name}?sslmode=require"
  })
  secret_string_wo_version = var.password_version
}

# The task that prepares the database -----------------------------------------

resource "aws_cloudwatch_log_group" "task" {
  name              = "/${var.environment}/db"
  retention_in_days = var.log_retention_days
}

# The task reaches the cluster, and the AWS services through the VPC
# endpoints of the network: Amazon ECR for the image, Amazon S3 for its
# layers, CloudWatch Logs, and Secrets Manager. No rule names an address
# range: the private subnets have no route to the internet.
resource "aws_security_group" "task" {
  name        = "${var.environment}-db-task"
  description = "The task that prepares the database: PostgreSQL to the cluster, HTTPS to the AWS services."
  vpc_id      = var.vpc_id

  tags = {
    Name = "${var.environment}-db-task"
  }
}

resource "aws_vpc_security_group_egress_rule" "task_postgres" {
  security_group_id            = aws_security_group.task.id
  description                  = "PostgreSQL to the Aurora cluster."
  referenced_security_group_id = aws_security_group.cluster.id
  from_port                    = local.port
  to_port                      = local.port
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "task_endpoints" {
  security_group_id            = aws_security_group.task.id
  description                  = "HTTPS to the interface endpoints of the AWS services."
  referenced_security_group_id = var.endpoints_security_group_id
  from_port                    = 443
  to_port                      = 443
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "task_s3" {
  security_group_id = aws_security_group.task.id
  description       = "HTTPS to Amazon S3, for the layers of the image."
  prefix_list_id    = var.s3_prefix_list_id
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "cluster_from_task" {
  security_group_id            = aws_security_group.cluster.id
  description                  = "PostgreSQL from the task that prepares the database."
  referenced_security_group_id = aws_security_group.task.id
  from_port                    = local.port
  to_port                      = local.port
  ip_protocol                  = "tcp"
}

# ECS assumes both roles for the task.
data "aws_iam_policy_document" "ecs_tasks_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# What ECS needs to start the task: pull the image, write the logs, and read
# exactly the secrets that become its environment.
data "aws_iam_policy_document" "execution" {
  # Amazon ECR grants a token for the whole registry; the action takes no
  # resource.
  statement {
    sid       = "SignInToTheRegistry"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  statement {
    sid = "PullTheImage"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:GetDownloadUrlForLayer",
    ]
    resources = [local.repository_arn]
  }

  statement {
    sid       = "WriteTheLogs"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.task.arn}:*"]
  }

  # The secrets use the key that Secrets Manager manages for the account,
  # which a role may use through Secrets Manager without a grant of its own.
  statement {
    sid       = "ReadTheSecrets"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = concat([local.master_secret_arn], [for secret in aws_secretsmanager_secret.role : secret.arn])
  }
}

resource "aws_iam_role" "execution" {
  name               = "${var.environment}-db-execution"
  description        = "Starts the task that prepares the database of ${var.environment}."
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

resource "aws_iam_role_policy" "execution" {
  name   = "start-the-task"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.execution.json
}

# The script calls no AWS API, so the role of the task has no permission.
resource "aws_iam_role" "task" {
  name               = "${var.environment}-db-task"
  description        = "The task that prepares the database of ${var.environment}. It calls no AWS API."
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

locals {
  # The environment of db/prepare.sh. The secrets fill the rest: the master
  # user from the secret of RDS, and the password of each role from its
  # secret. ECS reads one key of a JSON secret with "<ARN>:<key>::".
  container = {
    name                   = local.container_name
    image                  = var.db_image
    essential              = true
    readonlyRootFilesystem = true
    environment = [
      { name = "PGHOST", value = aws_rds_cluster.this.endpoint },
      { name = "PGPORT", value = tostring(local.port) },
      { name = "PGDATABASE", value = var.database_name },
      { name = "PGSSLMODE", value = "require" },
    ]
    secrets = concat(
      [
        { name = "PGUSER", valueFrom = "${local.master_secret_arn}:username::" },
        { name = "PGPASSWORD", valueFrom = "${local.master_secret_arn}:password::" },
      ],
      [
        for key, role in local.roles : {
          name      = "DB_${upper(role.kind)}_PASSWORD_${upper(role.context)}"
          valueFrom = "${aws_secretsmanager_secret.role[key].arn}:password::"
        }
      ],
    )
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.task.name
        awslogs-region        = data.aws_region.current.region
        awslogs-stream-prefix = local.container_name
      }
    }
  }
}

# The deploy runs this task once before the services start (output db_task),
# with `aws ecs run-task`. It writes no file, so its root file system is
# read-only.
resource "aws_ecs_task_definition" "db" {
  family                   = "${var.environment}-db"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn
  container_definitions    = jsonencode([local.container])

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = var.task_cpu_architecture
  }
}
