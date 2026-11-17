# The room is booked for the period

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 409.

## What it means

Another confirmed reservation of the same room overlaps the period. Two periods that only touch do not overlap: one reservation may end when the next one starts.

## What to do

List the reservations of the room for that day, and choose a free period or another room.
