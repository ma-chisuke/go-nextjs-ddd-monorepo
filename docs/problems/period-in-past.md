# The period starts in the past

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

A reservation cannot start before the moment the request arrives. The rule also applies to a change of a reservation: a reservation that has started cannot be rescheduled.

## What to do

Choose a period that starts later than now.
