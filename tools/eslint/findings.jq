# One finding per line from `eslint --format json`. $root is the absolute path
# of the repository, with a trailing slash, so that paths are printed relative.
.[] | (.filePath | ltrimstr($root)) as $file
| .messages[]
| "\($file):\(.line // 1): [\(.ruleId // "eslint")] \(.message | gsub("\n"; " "))"
