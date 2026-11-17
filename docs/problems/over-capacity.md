# The attendees do not fit in the room

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

The number of attendees is larger than the capacity of the room. The Facility API states the capacity of each room.

## What to do

Book a room whose capacity holds the attendees, or send a smaller `attendees`.
