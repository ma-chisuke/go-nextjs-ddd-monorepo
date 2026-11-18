import assert from "node:assert/strict";
import { test } from "node:test";

import type { Problem } from "../../lib/problem.ts";
import { reservationProblem } from "./problems.ts";

function problem(kind: string, status: number, extra: Partial<Problem> = {}): Problem {
  return {
    type: `https://github.com/example/repo/blob/main/docs/problems/${kind}.md`,
    title: `Title of ${kind}`,
    status,
    ...extra,
  };
}

test("each rule of a period gets its own words, about the start and the end", () => {
  for (const [kind, title] of [
    ["room-already-booked", "The room is already booked for part of this period."],
    ["outside-opening-hours", "The room is not open for the whole period."],
    ["period-misaligned", "The period must start and end on a quarter hour."],
    ["period-too-long", "A reservation lasts at most 4 hours."],
  ] as const) {
    const words = reservationProblem(problem(kind, 422), "book");
    assert.equal(words.notice?.title, title, kind);
    assert.deepEqual(words.noticeFields, ["start", "end"], kind);
    assert.equal(words.stale, false);
  }
});

test("the rules of the attendees show at the attendees", () => {
  assert.equal(
    reservationProblem(problem("over-capacity", 422), "book").fieldErrors.attendees,
    "More people attend than the room holds.",
  );
  assert.match(
    reservationProblem(problem("no-attendees", 422), "reschedule").fieldErrors.attendees ?? "",
    /At least 1 person/,
  );
});

test("an end before the start shows at the end", () => {
  assert.deepEqual(reservationProblem(problem("period-out-of-order", 422), "book").fieldErrors, {
    end: "The end must be later than the start.",
  });
});

test("a period in the past reads differently for a booking and a rescheduling", () => {
  assert.equal(
    reservationProblem(problem("period-in-past", 422), "book").notice?.title,
    "The period starts in the past.",
  );
  assert.equal(
    reservationProblem(problem("period-in-past", 422), "reschedule").notice?.title,
    "The period starts in the past, or the reservation has already started.",
  );
});

test("a Facility that does not answer says that nothing happened", () => {
  assert.match(
    reservationProblem(problem("facility-unavailable", 503), "book").notice?.detail ?? "",
    /Nothing was booked/,
  );
  assert.match(
    reservationProblem(problem("facility-unavailable", 503), "reschedule").notice?.detail ?? "",
    /Nothing was changed/,
  );
});

test("a reservation that changed meanwhile is stale", () => {
  assert.equal(reservationProblem(problem("precondition-failed", 412), "cancel").stale, true);
  assert.equal(reservationProblem(problem("reservation-cancelled", 409), "reschedule").stale, true);
  assert.equal(
    reservationProblem(problem("cancellation-too-late", 422), "cancel").notice?.title,
    "It is too late to cancel this reservation.",
  );
});

test("the parts of a validation error show at their fields", () => {
  const words = reservationProblem(
    problem("validation-error", 422, {
      errors: [
        { pointer: "/organizerEmail", detail: "must match the pattern" },
        { pointer: "/endsAt", detail: "must be a date-time" },
      ],
    }),
    "book",
  );
  assert.deepEqual(words.fieldErrors, {
    organizerEmail: "must match the pattern",
    end: "must be a date-time",
  });
  assert.equal(words.notice, undefined);
});

test("any other problem shows its title and detail as they are", () => {
  const words = reservationProblem(
    problem("internal-error", 500, { detail: "Try again later." }),
    "book",
  );
  assert.deepEqual(words.notice, { title: "Title of internal-error", detail: "Try again later." });
  const unknown = reservationProblem(
    problem("validation-error", 422, {
      errors: [{ pointer: "/roomId", detail: "must be a uuid" }],
    }),
    "book",
  );
  assert.equal(unknown.notice?.title, "Title of validation-error");
});
