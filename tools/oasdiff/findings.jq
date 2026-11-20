# One finding per line from `oasdiff breaking --format json`, for the errors
# (level 3) and the warnings (level 2): oasdiff asks for a raised version for
# both, so both fail the check. A change in the working tree has its line there (revisionSource); a
# removal has a line only in the version at the base, so the finding names
# line 1 and says where it was.
# Arguments: $doc, the document in the working tree; $base, the base commit.
.[] | select(.level >= 2)
| "\($doc):\(.revisionSource.line // 1): [\(.id)] "
  + (if (.operation // "") != "" then "\(.operation) \(.path): " else "" end)
  + .text
  + (if .revisionSource == null and .baseSource != null then " (line \(.baseSource.line) at \($base))" else "" end)
  + ". Fix: keep what existing clients rely on (add instead of removing, renaming, or requiring);"
  + " for a deliberate breaking change, raise info.version (below 1.0 the minor version, from 1.0 the major)"
  + " as docs/guides/api-changes.md describes. See TPL-0007."
