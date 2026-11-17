# One finding per line from `squawk --reporter json` on the DDL of a pgschema
# plan. The plan is generated, so each finding quotes its statement and names
# the schema.sql to change. squawk counts lines from zero, so one is added.
#
# Arguments:
#   $schema     the schema.sql of the plan
#   $sql        the text of the plan
#   $allow      the text of the allowance file of the context, or ""
#   $allowfile  the path of that file
#
# An allowance is a line `<rule> | <statement> | <reason>`: a dangerous
# statement that a person reviewed and lets through, for this change only. A
# finding with the same rule and statement is dropped. An allowance that
# matches no finding of the plan is itself a finding, so allowances do not
# pile up: the next change to the schema has to remove them.
def clean: gsub("^\\s+|;?\\s*$"; "");

($sql | split("\n")) as $lines
| ($allow | split("\n") | to_entries
   | map(select(.value | test("^\\s*(#.*)?$") | not))
   | map({line: (.key + 1), parts: (.value | split(" | "))})) as $entries
| ($entries | map(select((.parts | length) < 3 or (.parts[2:] | join(" | ") | test("^\\s*$"))))) as $malformed
| ($entries - $malformed | map({line, rule: (.parts[0] | clean), statement: (.parts[1] | clean)})) as $allowed
| [.[] | . + {statement: ($lines[.line] // "" | clean)}] as $findings
| (
    $findings[]
    | . as $f
    | select($allowed | any(.rule == $f.rule_name and .statement == $f.statement) | not)
    | "\(.file):\(.line + 1): [\(.rule_name)] \(.message): `\(.statement)`. Fix: "
      + (if .help then "\(.help), in \($schema), or make the change in several steps"
         else "make the change in \($schema) in several steps, for example add a new column and copy the data instead of changing a column" end)
      + " (docs/guides/change-a-schema.md). See TPL-0010."
  ),
  (
    $allowed[]
    | . as $a
    | select($findings | any(.rule_name == $a.rule and .statement == $a.statement) | not)
    | "\($allowfile):\(.line): [sql-plan/unused-allowance] no statement of this plan is `\(.statement)` with the finding \(.rule). Fix: remove the line; an allowance is for one change, and that change is done. See TPL-0010."
  ),
  (
    $malformed[]
    | "\($allowfile):\(.line): [sql-plan/allowance-format] an allowance is `<rule> | <statement> | <reason>`, and the reason is not optional. Fix: write the line in that form, with the statement exactly as the finding quotes it. See TPL-0010."
  )
