# The mutants of a JSON report of StrykerJS, with names that survive an edit
# elsewhere in the file (tools/scripts/mutation.sh). findings.jq includes this
# module, and so does the run of one mutant.
#
# Stryker numbers its mutants in sequence over the whole run, so that number
# changes with any edit. A mutant is named here as gomutants names one in Go:
# <file>:<declaration>:<Mutator>#<n>, the n-th mutant of that mutator in that
# top-level declaration (a function, or a constant such as a table of words),
# counted in the order of the source.

# A top-level declaration starts at the start of a line, because Prettier
# indents everything inside one.
def declaration_pattern:
  "^(export\\s+)?(default\\s+)?(declare\\s+)?(async\\s+)?(function\\*?|const|let|var|class|interface|type|enum)\\s+(?<name>[A-Za-z_$][A-Za-z0-9_$]*)";

# The name of the top-level declaration that holds each line of a source, by
# the index of the line, or "(module)" before the first declaration.
def declarations:
  split("\n")
  | reduce .[] as $line ([];
      . + [if $line | test(declaration_pattern)
           then $line | capture(declaration_pattern).name
           else .[-1] // "(module)" end]);

# The lines of a declaration, as a mutation range of Stryker: "<first>-<last>".
def lines_of($name):
  [declarations | to_entries[] | select(.value == $name) | .key + 1]
  | if length == 0 then empty else "\(.[0])-\(.[-1])" end;

def squash: (. // "") | gsub("\\s+"; " ") | gsub("^ | $"; "");

# A long piece of code, such as a whole block, shortened to its ends.
def short: if length > 80 then .[0:50] + " ... " + .[-25:] else . end;

# The text of a source between two positions of Stryker: lines and columns
# from 1, the end excluded.
def text($lines; $location):
  $location.start as $s | $location.end as $e
  | if $s.line == $e.line then $lines[$s.line - 1][$s.column - 1:$e.column - 1]
    else [$lines[$s.line - 1][$s.column - 1:]] + $lines[$s.line:$e.line - 1] + [$lines[$e.line - 1][:$e.column - 1]]
      | join("\n")
    end;

# Every mutant of a report, with its name, the file relative to the
# repository ($dir is the directory that Stryker ran in, with a trailing
# slash), its line, and its change as `original -> replacement`.
def mutants($dir):
  [.files | to_entries[]
   | ($dir + .key) as $file
   | (.value.source | split("\n")) as $lines
   | (.value.source | declarations) as $declarations
   | [.value.mutants[]
      | . + {file: $file, line: .location.start.line,
             declaration: $declarations[.location.start.line - 1],
             original: (text($lines; .location) | squash | short),
             replacement: (.replacement | squash | short)}]
   | group_by([.declaration, .mutatorName])[]
   | sort_by(.location.start.line, .location.start.column, .location.end.line, .location.end.column, (.id | tonumber))
   | to_entries[]
   | .value + {id: "\(.value.file):\(.value.declaration):\(.value.mutatorName)#\(.key + 1)",
               change: "\(.value.original) -> \(.value.replacement)"}];
