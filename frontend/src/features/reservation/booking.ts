// The form that books a room: what it holds, how the Server Function reads
// it, and which Idempotency-Key each submit sends. Pure logic, so that the
// unit tests run it without Next.js (make test-frontend).
//
// The Idempotency-Key. The Reservation API books once for each key: a request
// that comes again with the same key and the same body gets the reservation of
// the first one, and the same key with another body is refused
// (docs/problems/idempotency-key-reused.md). So a key stands for one booking
// that a person means, and it lives in the form, as a hidden field:
//
// - The server makes the first key when it renders the form, at request time
//   (BookingSection). Every submit of that form sends it, so a double click,
//   or a submit sent again after a timeout, books the room once.
// - After a submit, the form keeps the key and a fingerprint of the booking
//   that went out with it (`sentWith`). When the next submit asks for another
//   booking, because the person changed the times, the attendees, or the
//   email, it gets a new key (`chooseKey`); the same booking keeps the key.
//   The Server Function decides this from the fields that come with the
//   submit, so the rule holds without JavaScript in the browser too.
// - After a booking, the page moves to the reservation, and a form that is
//   rendered again starts with a new key.
// - A form can still carry a key that booked something else: a page that the
//   browser brings back from its history. The API then answers that the key
//   came with another request, or with the reservation of that key, which
//   may be cancelled by now. Either way the person asks for a new booking,
//   and the Server Function sends it once more with a new key
//   (`needsNewKey`).

import {
  formText,
  isEmailAddress,
  isUuid,
  wholeNumber,
  type FieldErrors,
  type Notice,
} from "../../lib/input.ts";
import type { components } from "../../lib/api/reservation.gen.ts";
import { problemKind, type Problem, type Result } from "../../lib/problem.ts";
import { isDate, isTimeZone } from "../../lib/zoned-time.ts";
import { readPeriod } from "./period.ts";
import { reservationProblem, type ReservationField } from "./problems.ts";

/** What the Reservation API needs to book a room. */
export type NewReservation = components["schemas"]["NewReservation"];

type Reservation = components["schemas"]["Reservation"];

/** What the person entered in each visible field. */
interface BookingValues {
  start: string;
  end: string;
  attendees: string;
  organizerEmail: string;
}

/** What the booking form shows, and the key that its next submit sends. */
export interface BookingState {
  /** The Idempotency-Key that the next submit of the form sends. */
  idempotencyKey: string;
  /** The fingerprint of the booking that last went out with the key, if one did. */
  sentWith?: string;
  /** What the person entered, to fill the form again. */
  values?: BookingValues;
  fieldErrors: FieldErrors<ReservationField>;
  /** A message about the whole form. */
  notice?: Notice;
  /** The fields that the notice is about. */
  noticeFields?: ReservationField[];
}

/** The state of a booking form that nobody has submitted yet, with its first key. */
export function blankBooking(idempotencyKey: string): BookingState {
  return { idempotencyKey, fieldErrors: {} };
}

/** A booking that the form asks for, with the key to send it with. */
export interface Booking {
  request: NewReservation;
  values: BookingValues;
  idempotencyKey: string;
  fingerprint: string;
}

// The limits of the API document.
const mostAttendees = 1000;

/**
 * The booking that a submitted form asks for, or what the form shows when a
 * field does not fit. `newKey` makes an Idempotency-Key, such as a random
 * UUID.
 */
export function readBooking(
  form: FormData,
  newKey: () => string,
): { ok: true; booking: Booking } | { ok: false; state: BookingState } {
  const roomId = formText(form, "roomId");
  const date = formText(form, "date") ?? "";
  const timeZone = formText(form, "timeZone") ?? "";
  const sentWith = formText(form, "sentWith");
  const submitted = {
    key: formText(form, "idempotencyKey"),
    sentWith: sentWith === "" ? undefined : sentWith,
  };
  const values: BookingValues = {
    start: formText(form, "start") ?? "",
    end: formText(form, "end") ?? "",
    attendees: formText(form, "attendees") ?? "",
    organizerEmail: formText(form, "organizerEmail") ?? "",
  };
  const failed = (
    fieldErrors: FieldErrors<ReservationField>,
    notice?: Notice,
  ): { ok: false; state: BookingState } => ({
    ok: false,
    state: {
      // Nothing went out: the key stays bound to what went out with it before.
      idempotencyKey:
        submitted.key !== undefined && isIdempotencyKey(submitted.key) ? submitted.key : newKey(),
      sentWith: submitted.sentWith,
      values,
      fieldErrors,
      notice,
    },
  });
  if (!isUuid(roomId) || !isDate(date) || !isTimeZone(timeZone)) {
    return failed(
      {},
      {
        title: "This form cannot be read.",
        detail: "Reload the page, and fill in the form again.",
      },
    );
  }
  const errors: FieldErrors<ReservationField> = {};
  const period = readPeriod(date, values.start, values.end, timeZone);
  if (!period.ok) {
    Object.assign(errors, period.fieldErrors);
  }
  const attendees = wholeNumber(values.attendees, 1, mostAttendees);
  if (attendees === undefined) {
    errors.attendees = `Enter a whole number from 1 to ${String(mostAttendees)}.`;
  }
  if (!isEmailAddress(values.organizerEmail)) {
    errors.organizerEmail = "Enter an email address, such as organizer@example.com.";
  }
  if (!period.ok || attendees === undefined || Object.keys(errors).length > 0) {
    return failed(errors);
  }
  const request: NewReservation = {
    roomId,
    startsAt: period.startsAt.toISOString(),
    endsAt: period.endsAt.toISOString(),
    attendees,
    organizerEmail: values.organizerEmail,
  };
  const fingerprint = bookingFingerprint(request);
  return {
    ok: true,
    booking: {
      request,
      values,
      fingerprint,
      idempotencyKey: chooseKey(submitted, fingerprint, newKey),
    },
  };
}

/**
 * The key for a booking: the key of the form, unless the form sent another
 * booking with it before, or carries no key that the API accepts.
 */
export function chooseKey(
  submitted: { key?: string | undefined; sentWith?: string | undefined },
  fingerprint: string,
  newKey: () => string,
): string {
  if (submitted.key === undefined || !isIdempotencyKey(submitted.key)) {
    return newKey();
  }
  if (submitted.sentWith !== undefined && submitted.sentWith !== fingerprint) {
    return newKey();
  }
  return submitted.key;
}

/**
 * Whether the answer to a booking says that its key booked something else
 * before, so that the booking that the person asks for now needs a new key.
 */
export function needsNewKey(answer: Result<Reservation>): boolean {
  return answer.ok
    ? answer.value.status === "cancelled"
    : problemKind(answer.problem) === "idempotency-key-reused";
}

/** What the form shows after a booking that the API refused, or that got no answer. */
export function bookingFailed(problem: Problem, booking: Booking): BookingState {
  const words = reservationProblem(problem, "book");
  return {
    // The booking went out with the key, and may have been made even if no
    // answer came: the same booking, sent again, must send the same key.
    idempotencyKey: booking.idempotencyKey,
    sentWith: booking.fingerprint,
    values: booking.values,
    fieldErrors: words.fieldErrors,
    notice: words.notice,
    noticeFields: words.noticeFields,
  };
}

/** Whether a value can be an Idempotency-Key: 1 to 255 visible ASCII characters. */
export function isIdempotencyKey(value: string): boolean {
  return /^[!-~]{1,255}$/.test(value);
}

/**
 * What identifies a booking, as the API compares two requests with one key:
 * the room, the instants of the period, the attendees, and the organizer.
 * FNV-1a, because the fingerprint only tells two bookings of one form apart.
 */
export function bookingFingerprint(request: NewReservation): string {
  const content = JSON.stringify([
    request.roomId.toLowerCase(),
    new Date(request.startsAt).getTime(),
    new Date(request.endsAt).getTime(),
    request.attendees,
    request.organizerEmail,
  ]);
  let hash = 0x811c9dc5;
  for (let index = 0; index < content.length; index++) {
    hash ^= content.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193) >>> 0;
  }
  return hash.toString(16).padStart(8, "0");
}
