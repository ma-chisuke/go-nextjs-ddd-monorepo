# The roles that let GitHub Actions reach the account of an environment:
# infra/modules/github-oidc. Someone with administrator access applies this
# unit once for each account (docs/guides/deploy.md); afterwards it changes
# through pull requests like every other unit.

locals {
  system = read_terragrunt_config("${dirname(find_in_parent_folders("root.hcl"))}/system.hcl").locals
  env    = read_terragrunt_config(find_in_parent_folders("env.hcl")).locals
}

terraform {
  source = "${get_repo_root()}/infra/modules//github-oidc"
}

inputs = {
  environment  = local.env.name
  repository   = local.system.repository
  state_bucket = "${local.system.system}-${local.env.name}-terraform-state-${local.env.account_id}"
}
