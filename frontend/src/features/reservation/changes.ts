// The forms that reschedule and cancel a reservation: what they hold, and
// how the Server Functions read them. Pure logic, so that the unit tests run
// it without Next.js (make test-frontend).
//
// The version. Each change names the version of the reservation that the
// page showed, the ETag of getReservation, in If-Match. Both forms carry it
// as a hidden field, and a form without a version that the API document
// allows is refused here, so a change never goes out without If-Match (428).
// When the reservation changed meanwhile, the API answers 412, and the page
// shows the reservation as it is now instead of overwriting it.

import { formText, isUuid, wholeNumber, type FieldErrors, type Notice } from "../../lib/input.ts";
import type { components } from "../../lib/api/reservation.gen.ts";
import type { Problem } from "../../lib/problem.ts";
import { isDate, isTimeZone } from "../../lib/zoned-time.ts";
import { readPeriod } from "./period.ts";
import { reservationProblem, type Operation, type ReservationField } from "./problems.ts";

/** The parts of a reservation that a rescheduling changes. */
export type ReservationChanges = components["schemas"]["ReservationChanges"];

/** What the person entered in each visible field of the form that reschedules. */
export interface RescheduleValues {
  date: string;
  start: string;
  end: string;
  attendees: string;
}

/** What a form that changes a reservation shows after a submit. */
export interface ChangeState {
  /** What the person entered, to fill the form again. */
  values?: RescheduleValues;
  fieldErrors: FieldErrors<ReservationField>;
  /** A message about the whole form, when the change did not happen. */
  notice?: Notice;
  /** The fields that the notice is about. */
  noticeFields?: ReservationField[];
  /** What happened, when the change happened. */
  done?: string;
}

/** The state of a form that nobody has submitted yet. */
export const unchanged: ChangeState = { fieldErrors: {} };

/** The reservation that a form changes, and the version of it that the page showed. */
export interface Target {
  reservationId: string;
  version: string;
}

/** Whether a value is a version of a reservation as ETag gives it: a strong entity tag, such as "1" with its quotes. */
export function isVersion(value: string): boolean {
  return /^"[0-9]+"$/.test(value);
}

const outdated: Notice = {
  title: "This page is out of date.",
  detail: "Reload it to see the reservation as it is now, and make your change again.",
};

// The limits of the API document.
const mostAttendees = 1000;

/** The reservation and the version that a submitted form names, if it names both. */
function readTarget(form: FormData): Target | undefined {
  const reservationId = formText(form, "reservationId");
  const version = formText(form, "version");
  return isUuid(reservationId) && isVersion(version) ? { reservationId, version } : undefined;
}

/**
 * The changes that a submitted form that reschedules asks for, or what the
 * form shows when it asks for none or a field does not fit. The form also
 * holds the period and the attendees as the page showed them, so that only
 * the parts that the person changed go to the API.
 */
export function readReschedule(
  form: FormData,
):
  | { ok: true; target: Target; changes: ReservationChanges; values: RescheduleValues }
  | { ok: false; state: ChangeState } {
  const target = readTarget(form);
  const timeZone = formText(form, "timeZone");
  const shown = {
    startsAt: formText(form, "shownStartsAt"),
    endsAt: formText(form, "shownEndsAt"),
    attendees: formText(form, "shownAttendees"),
  };
  const values: RescheduleValues = {
    date: formText(form, "date"),
    start: formText(form, "start"),
    end: formText(form, "end"),
    attendees: formText(form, "attendees"),
  };
  if (target === undefined || !isTimeZone(timeZone)) {
    return { ok: false, state: { values, fieldErrors: {}, notice: outdated } };
  }
  // The times are read on the date, so a date that does not fit leaves them unread.
  const period = isDate(values.date)
    ? readPeriod(values.date, values.start, values.end, timeZone)
    : undefined;
  const attendees = wholeNumber(values.attendees, 1, mostAttendees);
  if (period === undefined || !period.ok || attendees === undefined) {
    const errors: FieldErrors<ReservationField> = { ...period?.fieldErrors };
    if (period === undefined) {
      errors.date = "Enter a date, such as 2026-11-20.";
    }
    if (attendees === undefined) {
      errors.attendees = `Enter a whole number from 1 to ${String(mostAttendees)}.`;
    }
    return { ok: false, state: { values, fieldErrors: errors } };
  }
  const changes: ReservationChanges = {};
  if (period.startsAt.getTime() !== new Date(shown.startsAt).getTime()) {
    changes.startsAt = period.startsAt.toISOString();
  }
  if (period.endsAt.getTime() !== new Date(shown.endsAt).getTime()) {
    changes.endsAt = period.endsAt.toISOString();
  }
  if (String(attendees) !== shown.attendees) {
    changes.attendees = attendees;
  }
  if (Object.keys(changes).length === 0) {
    return {
      ok: false,
      state: {
        values,
        fieldErrors: {},
        notice: {
          title: "Nothing to change.",
          detail: "The period and the attendees are as the reservation has them.",
        },
      },
    };
  }
  return { ok: true, target, changes, values };
}

/** The reservation that a submitted form that cancels names, or what the form shows when it names none. */
export function readCancellation(
  form: FormData,
): { ok: true; target: Target } | { ok: false; state: ChangeState } {
  const target = readTarget(form);
  return target === undefined
    ? { ok: false, state: { fieldErrors: {}, notice: outdated } }
    : { ok: true, target };
}

/**
 * What a form shows after a change that the API refused, and whether the
 * page must read the reservation again, because it is not what the page
 * showed.
 */
export function changeFailed(
  problem: Problem,
  operation: Exclude<Operation, "book">,
  values?: RescheduleValues,
): { state: ChangeState; stale: boolean } {
  const words = reservationProblem(problem, operation);
  return {
    stale: words.stale,
    state: {
      // A form whose reservation changed meanwhile starts again from the
      // reservation as it is now, so that nothing overwrites the other change.
      values: words.stale ? undefined : values,
      fieldErrors: words.fieldErrors,
      notice: words.notice,
      noticeFields: words.noticeFields,
    },
  };
}
