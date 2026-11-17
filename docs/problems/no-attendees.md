# Nobody attends

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

A reservation is for at least one person, the organizer included. The API document already requires `attendees` to be 1 or more, so a request usually gets a validation error before this problem.

## What to do

Send `attendees` of 1 or more.
