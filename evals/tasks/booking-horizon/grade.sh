#!/usr/bin/env bash
# The criteria of the task booking-horizon (evals/run.sh sources this file and
# calls grade_task in the copy of the repository).
#
# The task adds a business rule to the core context and a type of problem to
# its API. What it measures: such a change has more parts than the code that
# makes the request fail. The rule belongs in the domain, which may not read
# the clock (TPL-0004); the problem needs its page in both languages
# (TPL-0008, TPL-0021); the README of the context lists the rules and their
# problems; and the screen has words for each problem that it can meet.
# `make check` holds some of these, and the documents of the repository say
# the rest.

slug=period-too-far-ahead
api=backend/internal/reservation/internal/adapter/http

# The API does what was asked, seen from outside: the test of the task, in
# the package of the API tests.
rule_holds() {
  cp "$TASK_DIR/acceptance_test.go" "$api/eval_acceptance_test.go"
  local status=0
  (cd backend && go test -count=1 -run '^TestEvalBookingHorizon$' ./internal/reservation/internal/adapter/http/) || status=$?
  rm -f "$api/eval_acceptance_test.go"
  return "$status"
}

# The rule is in the domain, with a test there: a file of the domain and a
# test file of the domain changed.
rule_is_in_the_domain() {
  local changed
  changed=$(changed_paths | grep '^backend/internal/reservation/internal/domain/' || true)
  if ! grep -q -v '_test\.go$' <<< "$changed" || ! grep -q '_test\.go$' <<< "$changed"; then
    echo "the change holds no rule in the domain of Reservation, or no test of it there: $(printf '%s' "$changed" | tr '\n' ' ')"
    return 1
  fi
}

problem_page_in_both_languages() {
  if [ ! -s "docs/problems/$slug.md" ] || [ ! -s "docs/problems/$slug.ja.md" ]; then
    echo "docs/problems/$slug.md or its translation is missing"
    return 1
  fi
}

context_readme_names_the_rule() {
  if ! grep -q "$slug" docs/contexts/reservation/README.md || ! grep -q "$slug" docs/contexts/reservation/README.ja.md; then
    echo "docs/contexts/reservation/README.md or its translation does not name $slug"
    return 1
  fi
}

screen_has_words() {
  if ! grep -q "$slug" frontend/src/features/reservation/problems.ts; then
    echo "frontend/src/features/reservation/problems.ts has no words for $slug"
    return 1
  fi
}

grade_task() {
  criterion rule-holds rule_holds
  criterion rule-is-in-the-domain rule_is_in_the_domain
  criterion problem-page-in-both-languages problem_page_in_both_languages
  criterion context-readme-names-the-rule context_readme_names_the_rule
  criterion screen-has-words screen_has_words
}
