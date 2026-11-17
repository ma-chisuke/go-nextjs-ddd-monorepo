# It is too late to cancel the reservation

An API answers with this problem, in the form of RFC 9457, together with the HTTP status code 422.

## What it means

A reservation can be cancelled until 30 minutes before it starts. The request arrived later, so the reservation stays confirmed.

## What to do

Nothing in the API cancels the reservation now. Cancel earlier next time.
