# The room would close before it opens

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

`openFrom` must be earlier in the day than `openUntil`. A room cannot be open across midnight.

## What to do

Send an `openFrom` that is earlier than `openUntil`.
