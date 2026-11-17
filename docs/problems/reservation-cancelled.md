# The reservation is cancelled

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 409.

## What it means

The reservation was cancelled. A cancelled reservation is neither rescheduled nor cancelled again.

## What to do

To use the room after all, book it again, with a new `Idempotency-Key`.
