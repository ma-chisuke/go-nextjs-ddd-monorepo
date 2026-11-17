# The room is not open for the whole period

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

A room can be booked only while it is open: between its `openFrom` and `openUntil`, read in the time zone of the room. A room closes on the day it opens, so a period that runs past midnight in the time zone of the room is never inside its opening hours.

## What to do

Read the opening hours and the time zone of the room from the Facility API, and choose a period inside them.
