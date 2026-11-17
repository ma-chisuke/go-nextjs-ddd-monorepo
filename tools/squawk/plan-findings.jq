# One finding per line from `squawk --reporter json` on the DDL of a pgschema
# plan. The plan is generated, so each finding quotes its statement and names
# the schema.sql to change. squawk counts lines from zero, so one is added.
# Arguments: $schema, the schema.sql of the plan; $sql, the text of the plan.
($sql | split("\n")) as $lines
| .[]
| ($lines[.line] // "" | gsub("^\\s+|;?\\s*$"; "")) as $statement
| "\(.file):\(.line + 1): [\(.rule_name)] \(.message): `\($statement)`. Fix: "
  + (if .help then "\(.help), in \($schema), or make the change in several steps"
     else "make the change in \($schema) in several steps, for example add a new column and copy the data instead of changing a column" end)
  + ". See TPL-0010."
