# The idempotency key came with another request

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

The `Idempotency-Key` of the request came before with another body. A key stands for one booking: the same key with the same body returns the reservation of the first request, and the same key with another body is refused.

## What to do

Choose a new key for each booking, such as a random UUID. Send a key again only to repeat the same request, for example after a timeout.
