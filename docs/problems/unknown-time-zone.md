# The time zone is not known

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

`timeZone` must be the name of a time zone in the IANA time zone database, such as `Asia/Tokyo`.

## What to do

Send the name of an IANA time zone. An abbreviation such as `JST` or an offset such as `+09:00` is not a name.
