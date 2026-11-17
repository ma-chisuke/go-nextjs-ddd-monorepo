# tflint checks every Terraform module: every rule of the Terraform language
# ruleset, and the AWS ruleset. `make setup` installs the plugin pinned here.
# https://github.com/terraform-linters/tflint/blob/master/docs/user-guide/config.md
config {
  # Local modules are checked with the module that calls them; remote modules
  # are not downloaded.
  call_module_type = "local"
}

plugin "terraform" {
  enabled = true
  preset  = "all"
}

plugin "aws" {
  enabled = true
  version = "0.50.0"
  source  = "github.com/terraform-linters/tflint-ruleset-aws"
}
