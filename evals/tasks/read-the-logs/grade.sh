#!/usr/bin/env bash
# The criteria of the task read-the-logs (evals/run.sh sources this file and
# calls grade_task in the copy of the repository).
#
# The task changes one document. What it measures: a document in this
# repository has a Japanese translation next to it, which changes in the same
# commit and records the version of the English file that it translates
# (TPL-0021). `make check` fails on a translation that was left behind, so an
# agent that runs the checks finds out; an agent that does not, does not.

guide=docs/guides/run-locally.md
translation=docs/guides/run-locally.ja.md

# The section exists, in its place, and says what was asked.
english_has_the_section() {
  if ! grep -q '^## Read the logs$' "$guide"; then
    echo "$guide has no section \"Read the logs\""
    return 1
  fi
  # Among the headings, it is the one after "See the mail".
  if ! grep '^## ' "$guide" | grep -A 1 -x '## See the mail' | tail -n 1 | grep -q -x '## Read the logs'; then
    echo "the section \"Read the logs\" does not come right after \"See the mail\""
    return 1
  fi
  if ! grep -q 'docker compose logs --follow' "$guide" || ! grep -q 'jq \.' "$guide" || ! grep -q 'docker compose ps' "$guide"; then
    echo "the section lacks one of the three things it was asked to say"
    return 1
  fi
}

# The translation has the section too: one more heading than before, and the
# commands, which are not translated.
translation_has_the_section() {
  local before after
  before=$(git show graded:"$translation" | grep -c '^## ')
  after=$(grep -c '^## ' "$translation")
  if [ "$after" -ne $((before + 1)) ]; then
    echo "$translation has $after sections, and $((before + 1)) are expected: the new section is missing"
    return 1
  fi
  if ! grep -q 'docker compose logs --follow' "$translation" || ! grep -q 'jq \.' "$translation"; then
    echo "the new section of $translation lacks the commands"
    return 1
  fi
}

# The translation is in Japanese, not a copy of the English text.
translation_is_japanese() {
  if grep -q 'shows the log of one service' "$translation"; then
    echo "$translation holds the English sentences"
    return 1
  fi
}

grade_task() {
  criterion english-has-the-section english_has_the_section
  criterion translation-has-the-section translation_has_the_section
  criterion translation-is-japanese translation_is_japanese
}
