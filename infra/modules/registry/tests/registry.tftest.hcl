# What the module plans, without an AWS account: the provider is a mock, so
# `terraform test` needs no credentials and creates nothing.

mock_provider "aws" {}

variables {
  repositories = ["facility", "reservation", "notification", "web"]
}

run "one_repository_for_each_image" {
  command = plan

  assert {
    condition     = toset(keys(aws_ecr_repository.this)) == toset(["facility", "reservation", "notification", "web"])
    error_message = "Each name needs a repository of that name."
  }

  assert {
    condition     = alltrue([for name, repository in aws_ecr_repository.this : repository.name == name])
    error_message = "A repository is named as given."
  }
}

run "tags_are_immutable_and_images_are_scanned" {
  command = plan

  assert {
    condition     = alltrue([for repository in aws_ecr_repository.this : repository.image_tag_mutability == "IMMUTABLE"])
    error_message = "A tag must name one image for good."
  }

  assert {
    condition     = alltrue([for repository in aws_ecr_repository.this : repository.image_scanning_configuration[0].scan_on_push])
    error_message = "Each image must be scanned when it is pushed."
  }
}

run "old_images_expire" {
  command = plan

  assert {
    condition     = toset(keys(aws_ecr_lifecycle_policy.this)) == toset(keys(aws_ecr_repository.this))
    error_message = "Each repository needs a lifecycle policy."
  }

  assert {
    condition = alltrue([
      for policy in aws_ecr_lifecycle_policy.this :
      jsondecode(policy.policy).rules[0].selection == { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 30 }
      && jsondecode(policy.policy).rules[0].action.type == "expire"
    ])
    error_message = "By default, a repository keeps the newest 30 images, tagged or not, and expires the rest."
  }
}

run "the_number_of_kept_images_can_change" {
  command = plan

  variables {
    kept_images = 5
  }

  assert {
    condition     = jsondecode(aws_ecr_lifecycle_policy.this["web"].policy).rules[0].selection.countNumber == 5
    error_message = "With kept_images = 5, a repository keeps the newest five images."
  }
}

run "no_repository_is_rejected" {
  command = plan

  variables {
    repositories = []
  }

  expect_failures = [var.repositories]
}

run "a_name_in_capitals_is_rejected" {
  command = plan

  variables {
    repositories = ["Facility"]
  }

  expect_failures = [var.repositories]
}

run "keeping_no_image_is_rejected" {
  command = plan

  variables {
    kept_images = 0
  }

  expect_failures = [var.kept_images]
}
