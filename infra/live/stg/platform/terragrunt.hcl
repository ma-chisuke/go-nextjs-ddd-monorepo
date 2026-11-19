# A unit: one state of one environment. Every terragrunt.hcl of the tree is
# this same file. What the unit creates is in _units/, under the path of this
# directory inside its environment; what the environment is, in env.hcl.

include "root" {
  path = find_in_parent_folders("root.hcl")
}

include "unit" {
  path = "${dirname(find_in_parent_folders("root.hcl"))}/_units${trimprefix(get_terragrunt_dir(), dirname(find_in_parent_folders("env.hcl")))}.hcl"
}
