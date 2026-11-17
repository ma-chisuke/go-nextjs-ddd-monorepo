# The rooms cannot be checked now

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 503.

## What it means

To book or reschedule, the service asks the Facility API for the capacity and the opening hours of the room. The Facility API did not answer in time, or it failed. Nothing was booked or changed.

## What to do

Wait for the number of seconds in the `Retry-After` header, and send the same request again. A booking with the same `Idempotency-Key` is safe to repeat.
