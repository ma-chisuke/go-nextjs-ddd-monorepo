# The period does not end after it starts

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

The `endsAt` of the reservation is not later than its `startsAt`. A reservation ends after it starts, so it lasts at least one quarter hour.

## What to do

Send an `endsAt` that is later than `startsAt`.
