import assert from "node:assert/strict";
import { test } from "node:test";

import type { Problem } from "../../lib/problem.ts";
import {
  blankRegistration,
  readRegistration,
  registrationProblem,
  type RoomValues,
} from "./registration.ts";

function form(fields: Record<string, string>): FormData {
  const data = new FormData();
  for (const [name, value] of Object.entries(fields)) {
    data.set(name, value);
  }
  return data;
}

const filledIn = {
  name: " Aoba ",
  capacity: "8",
  openFrom: "09:00",
  openUntil: "18:00",
  timeZone: "Asia/Tokyo",
};

const values: RoomValues = { ...filledIn, name: "Aoba" };

function problem(kind: string, status: number, extra: Partial<Problem> = {}): Problem {
  return {
    type: `https://example.com/docs/problems/${kind}.md`,
    title: `Title of ${kind}`,
    status,
    ...extra,
  };
}

test("a filled-in form asks for the room", () => {
  assert.deepEqual(readRegistration(form(filledIn)), {
    ok: true,
    values,
    room: {
      name: "Aoba",
      capacity: 8,
      openFrom: "09:00",
      openUntil: "18:00",
      timeZone: "Asia/Tokyo",
    },
  });
});

test("a form that nobody has submitted shows no message", () => {
  assert.deepEqual(blankRegistration, { fieldErrors: {} });
});

test("each field that does not fit gets a message, and the form keeps what was entered", () => {
  const read = readRegistration(
    form({ name: "  ", capacity: "0", openFrom: "9", openUntil: "25:00", timeZone: "" }),
  );
  assert.equal(read.ok, false);
  assert.deepEqual(read.state.fieldErrors, {
    name: "Enter the name of the room.",
    capacity: "Enter a whole number from 1 to 1000.",
    openFrom: "Enter a time of day on a 24-hour clock, such as 09:00.",
    openUntil: "Enter a time of day on a 24-hour clock, such as 09:00.",
    timeZone: "Choose a time zone.",
  });
  assert.equal(read.state.values?.openFrom, "9");
});

test("a field that does not fit on its own gets the one message", () => {
  for (const [field, value] of [
    ["name", ""],
    ["capacity", "1001"],
    ["openUntil", "24:00"],
    ["timeZone", ""],
  ] as const) {
    const read = readRegistration(form({ ...filledIn, [field]: value }));
    assert.equal(read.ok, false, field);
    assert.deepEqual(Object.keys(read.state.fieldErrors), [field]);
  }
});

test("a form without its fields is not read as a room", () => {
  const read = readRegistration(new FormData());
  assert.equal(read.ok, false);
  assert.equal(Object.keys(read.state.fieldErrors).length, 5);
});

test("the name and the time zone may be as long as the API document allows", () => {
  const longest = { ...filledIn, name: "x".repeat(100), timeZone: "x".repeat(64) };
  assert.equal(readRegistration(form(longest)).ok, true);
  const read = readRegistration(form({ ...filledIn, name: "x".repeat(101) }));
  assert.equal(read.ok, false);
  assert.equal(read.state.fieldErrors.name, "Use at most 100 characters.");
  const zone = readRegistration(form({ ...filledIn, timeZone: "x".repeat(65) }));
  assert.equal(zone.ok, false);
  assert.equal(zone.state.fieldErrors.timeZone, "Choose a time zone.");
});

test("a taken name shows as a notice about the name", () => {
  const state = registrationProblem(problem("room-name-taken", 409), values);
  assert.equal(state.notice?.title, "Another room has this name.");
  assert.match(state.notice.detail ?? "", /“Aoba”/);
  assert.deepEqual(state.noticeFields, ["name"]);
  assert.deepEqual(state.values, values);
});

test("opening hours in the wrong order show at the closing time", () => {
  const state = registrationProblem(problem("invalid-opening-hours", 422), values);
  assert.match(state.fieldErrors.openUntil ?? "", /close later in the day/);
  assert.equal(state.notice, undefined);
});

test("an unknown time zone shows at the time zone", () => {
  const state = registrationProblem(problem("unknown-time-zone", 422), values);
  assert.match(state.fieldErrors.timeZone ?? "", /IANA/);
});

test("the parts of a validation error show at their fields", () => {
  const state = registrationProblem(
    problem("validation-error", 422, {
      errors: [
        { pointer: "/name", detail: "must be at most 100 characters" },
        { pointer: "/capacity", detail: "must be at most 1000" },
        { pointer: "/openFrom", detail: "must match the pattern" },
        { pointer: "/openUntil", detail: "must match the pattern too" },
        { pointer: "/timeZone", detail: "must be at most 64 characters" },
      ],
    }),
    values,
  );
  assert.deepEqual(state.fieldErrors, {
    name: "must be at most 100 characters",
    capacity: "must be at most 1000",
    openFrom: "must match the pattern",
    openUntil: "must match the pattern too",
    timeZone: "must be at most 64 characters",
  });
});

test("any other problem shows as it is", () => {
  const state = registrationProblem(
    problem("internal-error", 500, { detail: "Try again later." }),
    values,
  );
  assert.deepEqual(state.notice, { title: "Title of internal-error", detail: "Try again later." });
  assert.deepEqual(state.fieldErrors, {});
});
