# The service failed

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 500.

## What it means

The service could not handle the request because of a fault of its own. The response says nothing about the cause; the log of the service does.

## What to do

Try again later. If the problem stays, report it to the people who run the service, with the time of the request.
