# One finding per line from `squawk --reporter json`. squawk counts lines from
# zero in this report, so one is added.
.[] | "\(.file):\(.line + 1): [\(.rule_name)] \(.message)\(if .help then ". Fix: \(.help)" else "" end)"
