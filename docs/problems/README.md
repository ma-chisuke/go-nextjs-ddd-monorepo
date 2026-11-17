# Problems

When a request fails, an API of this repository answers with a problem in the form of RFC 9457 (`application/problem+json`).
The `type` member of a problem is a URI that leads to a page in this directory.
The page says what the problem means and what a client can do about it.

Every kind of problem that the code can answer with has a page here, and a test fails when one is missing (TPL-0008).
