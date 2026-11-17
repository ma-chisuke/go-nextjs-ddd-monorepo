# The period does not start and end on a quarter hour

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

Reservations follow a grid of 15 minutes. A period starts and ends at 0, 15, 30, or 45 minutes past an hour, with no seconds, such as 10:00 to 11:15.

## What to do

Round the start and the end of the period to a quarter hour, and send the request again.
