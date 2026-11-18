import assert from "node:assert/strict";
import { test } from "node:test";

import type { Problem } from "../../lib/problem.ts";
import { readRegistration, registrationProblem, type RoomValues } from "./registration.ts";

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

test("each field that does not fit gets a message, and the form keeps what was entered", () => {
  const read = readRegistration(
    form({ name: "  ", capacity: "0", openFrom: "9", openUntil: "25:00", timeZone: "" }),
  );
  assert.equal(read.ok, false);
  assert.deepEqual(Object.keys(read.state.fieldErrors).sort(), [
    "capacity",
    "name",
    "openFrom",
    "openUntil",
    "timeZone",
  ]);
  assert.equal(read.state.values?.openFrom, "9");
});

test("a form without its fields is not read as a room", () => {
  const read = readRegistration(new FormData());
  assert.equal(read.ok, false);
  assert.equal(Object.keys(read.state.fieldErrors).length, 5);
});

test("a name that is too long is refused before the API sees it", () => {
  const read = readRegistration(form({ ...filledIn, name: "x".repeat(101) }));
  assert.equal(read.ok, false);
  assert.equal(read.state.fieldErrors.name, "Use at most 100 characters.");
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
      errors: [{ pointer: "/capacity", detail: "must be at most 1000" }],
    }),
    values,
  );
  assert.deepEqual(state.fieldErrors, { capacity: "must be at most 1000" });
});

test("any other problem shows as it is", () => {
  const state = registrationProblem(
    problem("internal-error", 500, { detail: "Try again later." }),
    values,
  );
  assert.deepEqual(state.notice, { title: "Title of internal-error", detail: "Try again later." });
  assert.deepEqual(state.fieldErrors, {});
});
