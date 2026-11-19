# The Amazon ECR repositories of the images that an environment runs
# (TPL-0014): one repository for each image, such as the image of each
# context and that of the web front end.

resource "aws_ecr_repository" "this" {
  for_each = var.repositories

  name = each.key

  # A tag names one image for good. A task definition that names a tag runs
  # the image that was reviewed and deployed, and a push cannot change what a
  # rollback goes back to.
  image_tag_mutability = "IMMUTABLE"

  # ECR scans each image as it arrives, so a known vulnerability shows before
  # the image is deployed.
  image_scanning_configuration {
    scan_on_push = true
  }
}

# Every push adds an image, and storage is charged. The newest images stay,
# whatever their tags, and the rest expire.
resource "aws_ecr_lifecycle_policy" "this" {
  for_each = aws_ecr_repository.this

  repository = each.value.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep the newest ${var.kept_images} images."
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = var.kept_images
        }
        action = {
          type = "expire"
        }
      },
    ]
  })
}
