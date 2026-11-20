// The form that registers a room: what it holds, how a Server Function reads
// it, and what the screen says about each problem of the Facility API. Pure
// logic, so that the unit tests run it without Next.js (make test-frontend).
// See docs/contexts/facility/ for the rules of a room.

import {
  formText,
  problemFieldErrors,
  problemNotice,
  wholeNumber,
  type FieldErrors,
  type Notice,
} from "../../lib/input.ts";
import type { components } from "../../lib/api/facility.gen.ts";
import { problemKind, type Problem } from "../../lib/problem.ts";
import { isTimeOfDay } from "../../lib/zoned-time.ts";

/** What the Facility API needs to register a room. */
export type NewRoom = components["schemas"]["NewRoom"];

/** The fields of the form, named as the API names the parts of a new room. */
export type RoomField = "name" | "capacity" | "openFrom" | "openUntil" | "timeZone";

/** What the person entered in each field. */
export type RoomValues = Record<RoomField, string>;

/** What the form shows after a submit that did not register the room. */
export interface RegistrationState {
  /** What the person entered, to fill the form again. */
  values?: RoomValues;
  fieldErrors: FieldErrors<RoomField>;
  /** A message about the whole form. */
  notice?: Notice;
  /** The fields that the notice is about. */
  noticeFields?: RoomField[];
}

/** The state of a form that nobody has submitted yet. */
export const blankRegistration: RegistrationState = { fieldErrors: {} };

// The limits of the API document.
const longestName = 100;
const longestTimeZone = 64;
const largestCapacity = 1000;

/** The room that a submitted form asks for, or what the form shows when a field does not fit. */
export function readRegistration(
  form: FormData,
): { ok: true; room: NewRoom; values: RoomValues } | { ok: false; state: RegistrationState } {
  const values: RoomValues = {
    name: formText(form, "name"),
    capacity: formText(form, "capacity"),
    openFrom: formText(form, "openFrom"),
    openUntil: formText(form, "openUntil"),
    timeZone: formText(form, "timeZone"),
  };
  const errors: FieldErrors<RoomField> = {};
  if (values.name === "") {
    errors.name = "Enter the name of the room.";
  } else if (values.name.length > longestName) {
    errors.name = `Use at most ${String(longestName)} characters.`;
  }
  for (const field of ["openFrom", "openUntil"] as const) {
    if (!isTimeOfDay(values[field])) {
      errors[field] = "Enter a time of day on a 24-hour clock, such as 09:00.";
    }
  }
  if (values.timeZone === "" || values.timeZone.length > longestTimeZone) {
    errors.timeZone = "Choose a time zone.";
  }
  // The capacity comes last, because the room needs it as a number.
  const capacity = wholeNumber(values.capacity, 1, largestCapacity);
  if (capacity === undefined || Object.keys(errors).length > 0) {
    if (capacity === undefined) {
      errors.capacity = `Enter a whole number from 1 to ${String(largestCapacity)}.`;
    }
    return { ok: false, state: { values, fieldErrors: errors } };
  }
  return {
    ok: true,
    values,
    room: {
      name: values.name,
      capacity,
      openFrom: values.openFrom,
      openUntil: values.openUntil,
      timeZone: values.timeZone,
    },
  };
}

/**
 * What the form says about a problem of the Facility API. The kinds of
 * problem that a person can fix in the form get words of their own, at the
 * field that they are about; any other problem shows as it is.
 */
export function registrationProblem(problem: Problem, values: RoomValues): RegistrationState {
  switch (problemKind(problem)) {
    case "room-name-taken":
      return {
        values,
        fieldErrors: {},
        notice: {
          title: "Another room has this name.",
          detail: `A room called “${values.name}” exists already. Choose another name.`,
        },
        noticeFields: ["name"],
      };
    case "invalid-opening-hours":
      return {
        values,
        fieldErrors: {
          openUntil:
            "The room must close later in the day than it opens. No room is open across midnight.",
        },
      };
    case "unknown-time-zone":
      return {
        values,
        fieldErrors: {
          timeZone: "This is not a time zone of the IANA database. Choose one from the list.",
        },
      };
  }
  const errors = problemFieldErrors<RoomField>(problem, {
    name: "name",
    capacity: "capacity",
    openFrom: "openFrom",
    openUntil: "openUntil",
    timeZone: "timeZone",
  });
  if (Object.keys(errors).length > 0) {
    return { values, fieldErrors: errors };
  }
  return { values, fieldErrors: {}, notice: problemNotice(problem) };
}
