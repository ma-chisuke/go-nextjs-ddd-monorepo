# One finding per line from the JSON report of StrykerJS, for the modules of
# the frontend (tools/scripts/mutation.sh). Run with `-L tools/stryker`.
#
# Arguments:
#   $dir        the directory that Stryker ran in, with a trailing slash ("frontend/")
#   $full       "true" when the run mutated every module of its configuration,
#               "false" when it was narrowed to some of them
#   $allow      the text of the allowance file
#   $allowfile  the path of that file
#
# Mutants are named as mutants.jq names them. Every mutant that survived fails
# the check, and so does every mutant that no test runs. A mutant that timed
# out made a test hang, so a test noticed it. A mutant that breaks the test
# process (a run-time error) was noticed too; one that does not compile does
# not count.
#
# equivalent-mutants.allow holds the allowances of Go and of the frontend, in
# one format: `<mutant> | <original> -> <replacement> | <reason>`. A line
# belongs to this check when its mutant names a JavaScript or TypeScript file,
# and to the check of Go (tools/gomutants/findings.jq) otherwise. An allowance
# lets a mutant that a person showed to be equivalent survive. One that matches
# no surviving mutant is itself a finding, so allowances do not rot.
include "mutants";

def fileof: split(":")[0];
def script: fileof | test("\\.[cm]?[jt]sx?$");
def fix: "(docs/guides/mutation-tests.md). See TPL-0023.";

($allow | split("\n") | to_entries
 | map(select(.value | test("^\\s*(#.*)?$") | not))
 | map({line: (.key + 1), parts: (.value | split(" | "))})
 | map(select(.parts[0] | squash | script))) as $entries
| ($entries | map(select((.parts | length) < 3 or (.parts[-1] | test("^\\s*$"))))) as $malformed
| ($entries - $malformed
   | map({line, id: (.parts[0] | squash), change: (.parts[1:-1] | join(" | ") | squash)})) as $allowed
| [.files | keys[] | $dir + .] as $files
| mutants($dir) as $mutants
| (
    $mutants[]
    | . as $m
    | if .status == "Survived" then
        select($allowed | any(.id == $m.id and .change == $m.change) | not)
        | "\(.file):\(.line): [mutation/survived] every test passes with `\(.original)` changed to `\(.replacement)` (\(.mutatorName), mutant \(.id)). Fix: add a test that fails with this change, or make the code simpler; `make mutant-frontend ID='\(.id)'` runs it alone \(fix)"
      elif .status == "NoCoverage" then
        "\(.file):\(.line): [mutation/not-covered] no test runs this line, so none would notice `\(.original)` changed to `\(.replacement)` (\(.mutatorName), mutant \(.id)). Fix: add a test that runs it and checks what it does \(fix)"
      else empty end
  ),
  (
    $allowed[]
    | . as $a
    | (.id | fileof) as $file
    | if $files | any(. == $file) then
        select($mutants | any(.status == "Survived" and .id == $a.id and .change == $a.change) | not)
        | "\($allowfile):\(.line): [mutation/unused-allowance] no mutant that survived is \(.id) with `\(.change)`. Fix: remove the line if a test kills the mutant now or the code is gone; if the declaration changed, find the mutant in the findings and decide again whether it is equivalent \(fix)"
      elif $full == "true" then
        "\($allowfile):\(.line): [mutation/unused-allowance] test-mutation-frontend mutates no module \($file), so no mutant is \(.id): the module is gone, or has no unit test next to it. Fix: remove the line \(fix)"
      else empty end
  ),
  (
    $malformed[]
    | "\($allowfile):\(.line): [mutation/allowance-format] an allowance is `<mutant> | <original> -> <replacement> | <reason>`, and the reason is not optional. Fix: write the line in that form, with the mutant and the change as the finding names them \(fix)"
  )
