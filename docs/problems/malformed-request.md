# The request cannot be read

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 400.

## What it means

The service cannot read the request: the JSON of the body is broken, or a parameter cannot be decoded into its type.

## What to do

Send a body that is valid JSON, and parameters in the form that the API document gives. The `detail` member says which part cannot be read.
