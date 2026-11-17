# Nothing is at this URL

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 404.

## What it means

No operation of the API answers to the path of the request.

## What to do

Check the path against the API document. When a resource with an id is missing, the API answers with a problem of its own, for example `room-not-found`.
