# One finding per line from `terraform test -json`, which writes one JSON
# object per line: every diagnostic that is an error, with the run it belongs
# to. $dir is the module directory, relative to the repository, with a
# trailing slash.
select(.type == "diagnostic" and .diagnostic.severity == "error")
| "\($dir)\(.diagnostic.range.filename // .["@testfile"] // ""):\(.diagnostic.range.start.line // 1): [terraform test] "
  + (if .["@testrun"] then "run \"\(.["@testrun"])\": " else "" end)
  + .diagnostic.summary
  + (if (.diagnostic.detail // "") != "" then ": " + (.diagnostic.detail | gsub("\n"; " ")) else "" end)
