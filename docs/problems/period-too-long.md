# The period is longer than a reservation may last

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

A reservation holds a room for at most 4 hours, so that other people can use the room on the same day.

## What to do

Shorten the period to 4 hours or less.
