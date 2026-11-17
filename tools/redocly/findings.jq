# One finding per line from `redocly lint --format json`.
.problems[] | .location[0] as $at
| "\($at.source.ref):\($at.start.line // 1): [\(.ruleId)] \(.message | gsub("\n"; " "))"
