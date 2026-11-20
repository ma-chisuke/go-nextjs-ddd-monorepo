# One finding per line from the JSON report of gomutants (`-o`), for the
# packages of one Go module (tools/scripts/mutation.sh).
#
# Arguments:
#   $module     the directory of the module, with a trailing slash ("backend/")
#   $packages   the directories of the packages of the run, one per line
#   $allow      the text of the allowance file
#   $allowfile  the path of that file
#
# A mutant is named as gomutants names it, with the module in front:
# <file>:<function>:<TYPE>#<n>, the n-th place of that type in that function,
# so that its name survives a change elsewhere in the file.
#
# Every mutant that lived fails the check, and so does every mutant on a line
# that no test runs. A mutant that timed out made a test hang, so a test
# noticed it. An allowance is a line `<mutant> | <original> -> <replacement> |
# <reason>`: a mutant that a person showed to be equivalent, which no test can
# kill. It lets that mutant live. An allowance of a package of the run that
# matches no living mutant is itself a finding, so allowances do not rot.
def squash: (. // "") | gsub("\\s+"; " ") | gsub("^ | $"; "");
def change: "\(.original | squash) -> \(.replacement | squash)";
def fileof: split(":")[0];
def dir: sub("/[^/]*$"; "");
def fix: "(docs/guides/mutation-tests.md). See TPL-0023.";

($packages | split("\n") | map(select(. != ""))) as $pkgs
| ($allow | split("\n") | to_entries
   | map(select(.value | test("^\\s*(#.*)?$") | not))
   | map({line: (.key + 1), parts: (.value | split(" | "))})) as $entries
| ($entries | map(select((.parts | length) < 3 or (.parts[-1] | test("^\\s*$"))))) as $malformed
| ($entries - $malformed
   | map({line, id: (.parts[0] | squash), change: (.parts[1:-1] | join(" | ") | squash)})
   | map(select((.id | startswith($module)) and ((.id | fileof | dir) as $d | $pkgs | index($d))))) as $allowed
| [.files[].mutations[] | . + {id: ($module + .id), change: change}] as $mutants
| (
    $mutants[]
    | . as $m
    | (.id | fileof) as $file
    | if .status == "LIVED" then
        select($allowed | any(.id == $m.id and .change == $m.change) | not)
        | "\($file):\(.line): [mutation/survived] every test passes with `\(.original | squash)` changed to `\(.replacement | squash)` (\(.type), mutant \(.id)). Fix: add a test that fails with this change, or make the code simpler; `make mutant ID='\(.id)'` runs it alone \(fix)"
      elif .status == "NOT COVERED" then
        "\($file):\(.line): [mutation/not-covered] no test runs this line, so none would notice `\(.original | squash)` changed to `\(.replacement | squash)` (\(.type), mutant \(.id)). Fix: add a test that runs it and checks what it does \(fix)"
      elif .status == "INFRA ERROR" then
        "\($file):\(.line): [mutation/infra-error] gomutants could not test `\(.original | squash)` changed to `\(.replacement | squash)` (\(.type), mutant \(.id)), because the machine failed it. Fix: run the check again on a machine with free memory and disk \(fix)"
      else empty end
  ),
  (
    $allowed[]
    | . as $a
    | select($mutants | any(.status == "LIVED" and .id == $a.id and .change == $a.change) | not)
    | "\($allowfile):\(.line): [mutation/unused-allowance] no mutant that lived is \(.id) with `\(.change)`. Fix: remove the line if a test kills the mutant now or the code is gone; if the function changed, find the mutant in the findings and decide again whether it is equivalent \(fix)"
  ),
  (
    $malformed[]
    | "\($allowfile):\(.line): [mutation/allowance-format] an allowance is `<mutant> | <original> -> <replacement> | <reason>`, and the reason is not optional. Fix: write the line in that form, with the mutant and the change as the finding names them \(fix)"
  )
