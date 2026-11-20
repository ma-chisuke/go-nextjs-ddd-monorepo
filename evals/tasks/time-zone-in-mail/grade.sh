#!/usr/bin/env bash
# The criteria of the task time-zone-in-mail (evals/run.sh sources this file
# and calls grade_task in the copy of the repository).
#
# The task adds a property to an integration event, and makes its consumer
# use it. What it measures: a contract between two contexts changes in a way
# that the consumers of its version can still read, on both sides, and
# neither context imports the other (TPL-0005, TPL-0013). `make check` holds
# the form of the schema (lint-events), its compatibility (lint-events-
# breaking), the AsyncAPI documents (lint-contracts), and the imports
# (lint-arch); the acceptance tests hold what each side does, through what
# the other side sees: the JSON that Reservation publishes, and the mail that
# Notification sends.
#
# The payload exists as JSON only inside the store of Reservation, behind the
# outbox, so the side of Reservation is tested against a database of the
# grader's own; so is the side of Notification, whose handler records each
# mail in its database.

publisher=backend/internal/reservation/internal/adapter/postgres
consumer=backend/internal/notification/internal/app
contract=contracts/events/reservation/ReservationConfirmed.v1.json

# Reservation publishes the time zone of the room in each event
# ReservationConfirmed, the one of a booking and the one of a rescheduling,
# and each event matches its contract.
events_name_the_time_zone() {
  cp "$TASK_DIR/publisher_test.go" "$publisher/eval_publisher_integration_test.go"
  local status=0
  with_database go_test -count=1 -tags integration -run '^TestEvalConfirmedEventsNameTheTimeZone$' "./${publisher#backend/}/" || status=$?
  rm -f "$publisher/eval_publisher_integration_test.go"
  return "$status"
}

# Notification writes the period on the clock of the room, with the name of
# its time zone, and an event without the property still in UTC.
mail_in_the_time_zone() {
  cp "$TASK_DIR/mail_test.go" "$consumer/eval_mail_integration_test.go"
  local status=0
  with_database go_test -count=1 -tags integration -run '^TestEvalMail' "./${consumer#backend/}/" || status=$?
  rm -f "$consumer/eval_mail_integration_test.go"
  return "$status"
}

# The JSON Schema of version 1 has the property and does not require it: the
# events of before lack it, and the consumers of the version still read
# them.
contract_has_an_optional_property() {
  if ! jq -e '(.properties | has("timeZone")) and ((.required // []) | index("timeZone") | not)' "$contract" > /dev/null; then
    echo "$contract has no property timeZone, or requires it"
    return 1
  fi
}

reservation_and_notification_integration_tests() {
  integration_tests internal/reservation && integration_tests internal/notification
}

# The integration tests of both contexts, which `make check-all` runs, pass
# with the new data of the event.
integration_tests_pass() {
  with_database reservation_and_notification_integration_tests
}

grade_task() {
  criterion events-name-the-time-zone events_name_the_time_zone
  criterion mail-in-the-time-zone mail_in_the_time_zone
  criterion contract-has-an-optional-property contract_has_an_optional_property
  criterion integration-tests-pass integration_tests_pass
}
