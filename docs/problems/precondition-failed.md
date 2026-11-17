# The resource changed since it was read

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 412.

## What it means

The `If-Match` header of the request names a version of the resource that is no longer the current one. Somebody changed the resource after the client read it.

## What to do

Read the resource again, decide whether the change still makes sense, and send the request with the new `ETag` in `If-Match`.
