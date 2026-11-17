# The reservation does not exist

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 404.

## What it means

No reservation has the id in the URL.

## What to do

List the reservations to find the id, or use the `Location` that the booking returned.
