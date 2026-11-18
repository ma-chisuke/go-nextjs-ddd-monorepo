"use server";

// The Server Functions of Reservation. Each one is a public entry point that
// anyone can post to: it narrows every value of the form before it calls the
// API, and it returns what the form shows for an expected failure instead of
// throwing. A form imports them, so they live outside server/.

import { refresh } from "next/cache";
import { redirect } from "next/navigation";

import { bookingFailed, needsNewKey, readBooking, type BookingState } from "./booking";
import { changeFailed, readCancellation, readReschedule, type ChangeState } from "./changes";
import {
  bookRoom as book,
  cancelReservation as cancel,
  rescheduleReservation as reschedule,
} from "./server/reservations";

// A new Idempotency-Key: a random UUID, as the API document suggests.
function newKey(): string {
  return crypto.randomUUID();
}

/**
 * Books the room that the form asks for, and goes to the page of the
 * reservation. booking.ts explains which Idempotency-Key each submit sends.
 */
export async function bookRoom(_previous: BookingState, form: FormData): Promise<BookingState> {
  const read = readBooking(form, newKey);
  if (!read.ok) {
    return read.state;
  }
  let { booking } = read;
  let answer = await book(booking.request, booking.idempotencyKey);
  if (needsNewKey(answer)) {
    // The key of the form booked something else before: this is a new booking.
    booking = { ...booking, idempotencyKey: newKey() };
    answer = await book(booking.request, booking.idempotencyKey);
  }
  if (!answer.ok) {
    return bookingFailed(answer.problem, booking);
  }
  redirect(`/reservations/${answer.value.id}?booked=1`);
}

/**
 * Reschedules the version of the reservation that the page showed. When the
 * reservation changed meanwhile, the page reads it again and shows it as it
 * is now.
 */
export async function rescheduleReservation(
  _previous: ChangeState,
  form: FormData,
): Promise<ChangeState> {
  const read = readReschedule(form);
  if (!read.ok) {
    return read.state;
  }
  const answer = await reschedule(read.target, read.changes);
  if (answer.ok) {
    refresh();
    return { fieldErrors: {}, done: "The reservation is rescheduled." };
  }
  const failed = changeFailed(answer.problem, "reschedule", read.values);
  if (failed.stale) {
    refresh();
  }
  return failed.state;
}

/**
 * Cancels the version of the reservation that the page showed. When the
 * reservation changed meanwhile, the page reads it again and shows it as it
 * is now.
 */
export async function cancelReservation(
  _previous: ChangeState,
  form: FormData,
): Promise<ChangeState> {
  const read = readCancellation(form);
  if (!read.ok) {
    return read.state;
  }
  const answer = await cancel(read.target);
  if (answer.ok) {
    refresh();
    return { fieldErrors: {}, done: "The reservation is cancelled." };
  }
  const failed = changeFailed(answer.problem, "cancel");
  if (failed.stale) {
    refresh();
  }
  return failed.state;
}
