# The cursor is not valid

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

The `cursor` parameter is not a value that this API returned as `nextCursor`.

## What to do

Leave `cursor` out to get the first page, and pass the `nextCursor` of each page unchanged to get the next one.
