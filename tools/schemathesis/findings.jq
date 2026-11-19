# One finding per line from the events that `schemathesis run --report ndjson`
# writes, read with --slurp (tools/scripts/api-fuzz.sh).
#
# $doc    the path of the API document
# $lines  the line of each operation in the document, by "<METHOD> <path>",
#         and the line of each path, by "<path>"
# $base   the base URL of the API, which the requests leave out
#
# Each check that fails becomes a finding, once for each operation, check, and
# kind of answer: a check that fails on many requests in the same way is one
# finding, with the first of those requests. Each error of the run, such as an
# API that does not answer, becomes a finding too.

def line($operation):
  $lines[$operation] // $lines[$operation | sub("^[A-Z]+ "; "")] // 1;

# The line of the path, for a check about the methods that a URL does not have.
def line($operation; $check):
  if $check | IN("unsupported_method", "allow_header_conformance")
  then line($operation | sub("^[A-Z]+ "; ""))
  else line($operation) end;

# Lines become sentences of one line.
def oneline: gsub("\\s*\n\\s*"; "\n") | gsub("\n"; "; ") | gsub("\\s+"; " ") | ltrimstr(" ") | rtrimstr(" ");

def sentence: if test("[.!?]$") then . else . + "." end;

def clip($n): if length > $n then .[0:$n] + "..." else . end;

# The text of a body, which the events hold in base64.
def text:
  if type == "object" and has("$base64") then .["$base64"] | @base64d
  elif type == "string" then .
  else "" end;

# The kind of a problem: the name of its page in docs/problems/.
def kind: if type == "string" then sub(".*/"; "") | sub("\\.md$"; "") else "" end;

# The answer of the API: its status, and for a problem its kind and detail.
def answer:
  if . == null then {status: null, kind: "", text: "no answer"}
  else
    (.content | text | try fromjson catch null) as $body
    | (if ($body | type) == "object" then $body.type | kind else "" end) as $kind
    | {
        status: .status_code,
        kind: $kind,
        text: ("\(.status_code)"
          + (if $kind != "" then " " + $kind else "" end)
          + (if $kind != "" and ($body.detail | type) == "string"
             then ": " + ($body.detail | oneline | clip(200)) else "" end))
      }
  end;

# The request: the method, the URL without its base, the headers that the
# test chose, and the body.
def request:
  if . == null then "none recorded"
  else
    .method + " " + (.uri | ltrimstr($base))
    + ([(.headers // {}) | to_entries[]
        | select(.key | ascii_downcase | IN("content-type", "if-match", "idempotency-key", "accept"))
        | " -H '\(.key): \(.value | if type == "array" then join(", ") else tostring end)'"]
       | join(""))
    + (if .body then " -d '\(.body | text | oneline | clip(300))'" else "" end)
  end;

(
  [ .[] | .ScenarioFinished? // empty
    | .recorder as $r
    | ($r.checks // {}) | to_entries[] | .key as $case
    | .value[] | select(.status == "failure")
    | .name as $check
    | .failure_info.failure as $f
    | ($r.interactions[$case] // null) as $i
    | ($i.response | answer) as $answer
    | {
        operation: ($f.operation // $r.label),
        check: $check,
        title: $f.title,
        code: $f.code,
        message: ($f.message // "" | oneline | clip(300)),
        answer: $answer,
        request: ($i.request | request)
      }
  ]
  | unique_by([.operation, .check, .code, .answer.status, .answer.kind])
  | .[]
  | "\($doc):\(line(.operation; .check)): [schemathesis/\(.check)] \(.operation): \(.title) (\(.code))"
    + (if .message == "" then "." else ": " + (.message | sentence) end)
    + " Answer: \(.answer.text | sentence) Request: \(.request)"
),
(
  [ .[] | .NonFatalError? // empty ]
  | unique_by([.label, .value.type])
  | .[]
  | "\($doc):\(line(.label // "")): [schemathesis/error] \(.label // "the run"): \(.value.type): \(.value.message | oneline | clip(300))"
)
