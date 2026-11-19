# What the module plans, without an AWS account: the provider is a mock, so
# `terraform test` needs no credentials and creates nothing. The policy
# documents are computed by the provider, so the tests look at the subjects
# that the module puts into them.

mock_provider "aws" {
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

  mock_resource "aws_iam_openid_connect_provider" {
    defaults = {
      arn = "arn:aws:iam::111111111111:oidc-provider/token.actions.githubusercontent.com"
    }
  }

  mock_data "aws_iam_openid_connect_provider" {
    defaults = {
      arn = "arn:aws:iam::111111111111:oidc-provider/token.actions.githubusercontent.com"
    }
  }
}

variables {
  environment  = "stg"
  repository   = "octo-org/octo-repo"
  state_bucket = "octo-terraform-state"
}

run "the_plan_role_is_for_pull_requests_and_read_only" {
  command = plan

  assert {
    condition     = output.plan_subject == "repo:octo-org/octo-repo:pull_request"
    error_message = "Only a job of a pull request of the repository may assume the read-only role."
  }

  assert {
    condition     = aws_iam_role_policy_attachment.plan_read_only.policy_arn == "arn:aws:iam::aws:policy/ReadOnlyAccess"
    error_message = "The role that plans must be read-only."
  }
}

run "the_deploy_role_is_for_one_github_environment" {
  command = plan

  assert {
    condition     = output.deploy_subject == "repo:octo-org/octo-repo:environment:stg"
    error_message = "Only a job of the GitHub Environment of this environment may assume the role that deploys."
  }

  assert {
    condition     = length(aws_iam_role_policy_attachment.deploy) == 1
    error_message = "The role that deploys gets the policies of deploy_policy_arns."
  }
}

run "the_immutable_form_of_a_repository_is_accepted" {
  command = plan

  variables {
    repository = "octo-org@123456/octo-repo@456789"
  }

  assert {
    condition     = output.deploy_subject == "repo:octo-org@123456/octo-repo@456789:environment:stg"
    error_message = "The subject must hold the repository as it was given."
  }
}

run "an_existing_provider_is_used" {
  command = plan

  variables {
    create_provider = false
  }

  assert {
    condition     = length(aws_iam_openid_connect_provider.github) == 0
    error_message = "With create_provider = false, the module creates no provider."
  }
}

run "a_wildcard_in_the_repository_is_rejected" {
  command = plan

  variables {
    repository = "octo-org/*"
  }

  expect_failures = [var.repository]
}

run "a_subject_smuggled_into_the_repository_is_rejected" {
  command = plan

  variables {
    repository = "octo-org/octo-repo:ref:refs/heads/main"
  }

  expect_failures = [var.repository]
}

run "a_role_without_policies_is_rejected" {
  command = plan

  variables {
    deploy_policy_arns = []
  }

  expect_failures = [var.deploy_policy_arns]
}
