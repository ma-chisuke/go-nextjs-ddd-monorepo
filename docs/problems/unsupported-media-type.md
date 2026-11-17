# The media type of the body is not supported

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 415.

## What it means

The `Content-Type` header of the request names a media type that the operation does not take.

## What to do

Send the body as `application/json`, and say so in `Content-Type`.
