# One finding per line from `terraform validate -json`. $dir is the module
# directory, relative to the repository, with a trailing slash.
.diagnostics[]
| "\($dir)\(.range.filename // ""):\(.range.start.line // 1): [terraform validate] \(.summary)\(if (.detail // "") != "" then ": " + (.detail | gsub("\n"; " ")) else "" end)"
