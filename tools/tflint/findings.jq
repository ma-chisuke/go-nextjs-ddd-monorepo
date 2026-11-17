# One finding per line from `tflint --format json`.
(.issues[]? | "\(.range.filename):\(.range.start.line): [\(.rule.name)] \(.message)"),
(.errors[]? | "\(.range.filename // ".tflint.hcl"):\(.range.start.line // 1): [tflint] \(.message | gsub("\n"; " "))")
