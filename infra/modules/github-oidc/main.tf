# How GitHub Actions reaches one AWS account without a stored key (TPL-0018).
# A job asks GitHub for a short-lived token that says which repository, and
# which pull request or GitHub Environment, the job belongs to. AWS exchanges
# the token for the credentials of a role, when the trust policy of the role
# names exactly that subject.
#
#   github-plan     read-only, for `terragrunt plan` on pull requests
#   github-deploy   applies changes, for the jobs of one GitHub Environment
#
# A trust policy that matches too much gives the role to code that nobody
# reviewed, so both policies compare the whole subject with StringEquals, and
# the repository is validated to hold no wildcard.

locals {
  issuer = "token.actions.githubusercontent.com"

  provider_arn = var.create_provider ? aws_iam_openid_connect_provider.github[0].arn : data.aws_iam_openid_connect_provider.github[0].arn

  # The subjects, as GitHub documents them. A job that names a GitHub
  # Environment has the environment in its subject, whatever started it.
  plan_subject   = "repo:${var.repository}:pull_request"
  deploy_subject = "repo:${var.repository}:environment:${var.environment}"
}

data "aws_partition" "current" {}

# AWS verifies the certificate of GitHub against its own trusted roots, so
# the provider needs no thumbprint.
resource "aws_iam_openid_connect_provider" "github" {
  count = var.create_provider ? 1 : 0

  url            = "https://${local.issuer}"
  client_id_list = ["sts.amazonaws.com"]
}

data "aws_iam_openid_connect_provider" "github" {
  count = var.create_provider ? 0 : 1

  url = "https://${local.issuer}"
}

# Plan ----------------------------------------------------------------------

data "aws_iam_policy_document" "plan_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.issuer}:aud"
      values   = ["sts.amazonaws.com"]
    }

    # A job of a pull request that names no GitHub Environment. GitHub gives
    # no token to a pull request from a fork.
    condition {
      test     = "StringEquals"
      variable = "${local.issuer}:sub"
      values   = [local.plan_subject]
    }
  }
}

resource "aws_iam_role" "plan" {
  name                 = "github-plan"
  description          = "Read-only: terragrunt plan on the pull requests of ${var.repository}."
  assume_role_policy   = data.aws_iam_policy_document.plan_assume.json
  max_session_duration = 3600
}

# ReadOnlyAccess reads the configuration of the resources and the state, and
# does not read the value of a secret.
resource "aws_iam_role_policy_attachment" "plan_read_only" {
  role       = aws_iam_role.plan.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/ReadOnlyAccess"
}

# A plan takes the lock of the state: a lock file next to the state, which
# Terraform writes and deletes (`use_lockfile`).
data "aws_iam_policy_document" "plan_lock" {
  statement {
    sid       = "LockTheState"
    actions   = ["s3:PutObject", "s3:DeleteObject"]
    resources = ["arn:${data.aws_partition.current.partition}:s3:::${var.state_bucket}/*.tflock"]
  }
}

resource "aws_iam_role_policy" "plan_lock" {
  name   = "lock-the-state"
  role   = aws_iam_role.plan.id
  policy = data.aws_iam_policy_document.plan_lock.json
}

# Deploy --------------------------------------------------------------------

data "aws_iam_policy_document" "deploy_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.issuer}:aud"
      values   = ["sts.amazonaws.com"]
    }

    # A job that names the GitHub Environment of this environment. The
    # settings of that Environment on GitHub decide which branch may use it,
    # and whether a person must approve the job first.
    condition {
      test     = "StringEquals"
      variable = "${local.issuer}:sub"
      values   = [local.deploy_subject]
    }
  }
}

resource "aws_iam_role" "deploy" {
  name                 = "github-deploy"
  description          = "Applies changes: the jobs of the GitHub Environment ${var.environment} of ${var.repository}."
  assume_role_policy   = data.aws_iam_policy_document.deploy_assume.json
  max_session_duration = 3600
}

resource "aws_iam_role_policy_attachment" "deploy" {
  for_each = toset(var.deploy_policy_arns)

  role       = aws_iam_role.deploy.name
  policy_arn = each.value
}
