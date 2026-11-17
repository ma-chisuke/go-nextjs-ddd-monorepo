# The request is not valid

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

The service can read the request, and a part of it breaks a rule of the API document: a required property is missing, a value is outside its limits, or a string does not have its format.

## What to do

Correct the parts that the `errors` member lists. Each entry has a `pointer` to the part (a JSON Pointer into the body, or the name of a parameter) and a `detail` that says which rule it breaks.
