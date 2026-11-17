# The request needs an If-Match header

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 428.

## What it means

The operation changes a resource, and the request does not say which version of the resource the client has seen. Without that, one client could overwrite the change of another without noticing.

## What to do

Read the resource, take the `ETag` header of the response, and send it in the `If-Match` header of the request.
