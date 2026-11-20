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

// The kinds of problem that get words of their own.
const kinds = [
  "room-already-booked",
  "outside-opening-hours",
  "period-misaligned",
  "period-too-long",
  "period-out-of-order",
  "period-in-past",
  "over-capacity",
  "no-attendees",
  "facility-unavailable",
  "cancellation-too-late",
  "precondition-failed",
  "reservation-cancelled",
];

test("every message about a problem is a sentence", () => {
  // The tests below hold the words that carry a fact (a number, a rule, or a
  // field) one by one; this one holds that no message is empty.
  for (const kind of kinds) {
    for (const operation of ["book", "reschedule", "cancel"] as const) {
      const words = reservationProblem(problem(kind, 422), operation);
      const messages = [
        words.notice?.title,
        words.notice?.detail,
        ...Object.values(words.fieldErrors),
      ];
      for (const message of messages.filter((text) => text !== undefined)) {
        assert.match(message, /^[A-Z].*\.$/, `${kind}, ${operation}`);
      }
    }
  }
});

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
  assert.equal(
    reservationProblem(problem("outside-opening-hours", 422), "book").notice?.detail,
    "Choose a period inside the opening hours of the room.",
  );
  assert.equal(
    reservationProblem(problem("period-misaligned", 422), "book").notice?.detail,
    "Use :00, :15, :30, or :45 past the hour, such as 10:00 to 11:15.",
  );
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
  assert.deepEqual(reservationProblem(problem("period-in-past", 422), "book"), {
    fieldErrors: {},
    notice: {
      title: "The period starts in the past.",
      detail: "Choose a period that starts later than now.",
    },
    noticeFields: ["start"],
    stale: false,
  });
  assert.deepEqual(reservationProblem(problem("period-in-past", 422), "reschedule"), {
    fieldErrors: {},
    notice: {
      title: "The period starts in the past, or the reservation has already started.",
      detail:
        "Choose a period that starts later than now. A reservation that has started cannot be rescheduled.",
    },
    noticeFields: ["start"],
    stale: false,
  });
});

test("a Facility that does not answer says that nothing happened", () => {
  const booking = reservationProblem(problem("facility-unavailable", 503), "book");
  assert.equal(
    booking.notice?.detail,
    "Nothing was booked. Wait a moment, then send the form again: sending it again never books the room twice.",
  );
  assert.deepEqual(booking.noticeFields, []);
  assert.equal(
    reservationProblem(problem("facility-unavailable", 503), "reschedule").notice?.detail,
    "Nothing was changed. Wait a moment, then send the form again.",
  );
});

test("a reservation that changed meanwhile is stale", () => {
  const changed = reservationProblem(problem("precondition-failed", 412), "cancel");
  assert.equal(changed.stale, true);
  assert.equal(
    changed.notice?.detail,
    "Nothing was changed. The page now shows the reservation as it is; make your change again if you still want it.",
  );
  const cancelled = reservationProblem(problem("reservation-cancelled", 409), "reschedule");
  assert.equal(cancelled.stale, true);
  assert.equal(
    cancelled.notice?.detail,
    "A cancelled reservation cannot be changed. Book the room again to use it.",
  );
  assert.deepEqual(reservationProblem(problem("cancellation-too-late", 422), "cancel").notice, {
    title: "It is too late to cancel this reservation.",
    detail: "A reservation can be cancelled until 30 minutes before it starts.",
  });
});

test("the parts of a validation error show at their fields", () => {
  const words = reservationProblem(
    problem("validation-error", 422, {
      errors: [
        { pointer: "/startsAt", detail: "must be a date-time" },
        { pointer: "/endsAt", detail: "must be later" },
        { pointer: "/attendees", detail: "must be at least 1" },
        { pointer: "/organizerEmail", detail: "must match the pattern" },
      ],
    }),
    "book",
  );
  assert.deepEqual(words, {
    fieldErrors: {
      start: "must be a date-time",
      end: "must be later",
      attendees: "must be at least 1",
      organizerEmail: "must match the pattern",
    },
    stale: false,
  });
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
