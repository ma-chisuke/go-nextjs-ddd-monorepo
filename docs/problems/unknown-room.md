# The room is not known

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

The Facility API has no room with the `roomId` of the request.

## What to do

List the rooms with the Facility API to find the id, or register the room there first.
