# The image repositories of an environment: infra/modules/registry. One for
# each bounded context, one for the web front end, and one for the image
# that prepares the database (tools/scripts/images.sh).

locals {
  system = read_terragrunt_config("${dirname(find_in_parent_folders("root.hcl"))}/system.hcl").locals
}

terraform {
  source = "${get_repo_root()}/infra/modules//registry"
}

inputs = {
  repositories = concat(local.system.contexts, ["web", "db"])
}
