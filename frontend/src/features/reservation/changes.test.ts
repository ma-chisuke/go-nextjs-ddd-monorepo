import assert from "node:assert/strict";
import { describe, test } from "node:test";

import type { Problem } from "../../lib/problem.ts";
import { changeFailed, isVersion, readCancellation, readReschedule } from "./changes.ts";

const reservationId = "01a1eeef-dde4-79b2-b195-89894d81fb15";

// A reservation from 10:00 to 11:00 in Tokyo for 4 people, as the page showed it.
const shown = {
  reservationId,
  version: '"3"',
  timeZone: "Asia/Tokyo",
  shownStartsAt: "2026-11-20T01:00:00Z",
  shownEndsAt: "2026-11-20T02:00:00Z",
  shownAttendees: "4",
  date: "2026-11-20",
  start: "10:00",
  end: "11:00",
  attendees: "4",
};

function form(fields: Record<string, string>): FormData {
  const data = new FormData();
  for (const [name, value] of Object.entries(fields)) {
    data.set(name, value);
  }
  return data;
}

function problem(kind: string, status: number): Problem {
  return {
    type: `https://example.com/docs/problems/${kind}.md`,
    title: `Title of ${kind}`,
    status,
  };
}

describe("readReschedule", () => {
  test("only the parts that the person changed go to the API", () => {
    const read = readReschedule(form({ ...shown, end: "11:30" }));
    assert.equal(read.ok, true);
    assert.deepEqual(read.target, { reservationId, version: '"3"' });
    assert.deepEqual(read.changes, { endsAt: "2026-11-20T02:30:00.000Z" });
  });

  test("another day and more attendees change the whole period and the attendees", () => {
    const read = readReschedule(form({ ...shown, date: "2026-11-21", attendees: "6" }));
    assert.equal(read.ok, true);
    assert.deepEqual(read.changes, {
      startsAt: "2026-11-21T01:00:00.000Z",
      endsAt: "2026-11-21T02:00:00.000Z",
      attendees: 6,
    });
  });

  test("a form that changes nothing sends nothing", () => {
    const read = readReschedule(form(shown));
    assert.equal(read.ok, false);
    assert.equal(read.state.notice?.title, "Nothing to change.");
  });

  test("each field that does not fit gets a message", () => {
    const read = readReschedule(form({ ...shown, date: "2026-11-31", attendees: "0" }));
    assert.equal(read.ok, false);
    assert.deepEqual(Object.keys(read.state.fieldErrors).sort(), ["attendees", "date"]);
    assert.equal(read.state.values?.date, "2026-11-31");
  });

  test("a form without the version that the page showed never goes out", () => {
    for (const version of ["", "3", 'W/"3"', '"x"']) {
      const read = readReschedule(form({ ...shown, version, end: "11:30" }));
      assert.equal(read.ok, false);
      assert.equal(read.state.notice?.title, "This page is out of date.");
    }
    const missing = form({ ...shown, end: "11:30" });
    missing.delete("version");
    assert.equal(readReschedule(missing).ok, false);
  });
});

describe("readCancellation", () => {
  test("a cancellation names the reservation and the version that the page showed", () => {
    const read = readCancellation(form({ reservationId, version: '"7"' }));
    assert.deepEqual(read, { ok: true, target: { reservationId, version: '"7"' } });
  });

  test("a cancellation without a version never goes out", () => {
    assert.equal(readCancellation(form({ reservationId })).ok, false);
    assert.equal(readCancellation(form({ reservationId: "x", version: '"7"' })).ok, false);
  });
});

describe("changeFailed", () => {
  test("a reservation that changed meanwhile is shown again as it is now", () => {
    const failed = changeFailed(problem("precondition-failed", 412), "reschedule", {
      date: "2026-11-20",
      start: "10:00",
      end: "11:30",
      attendees: "4",
    });
    assert.equal(failed.stale, true);
    assert.equal(failed.state.values, undefined);
    assert.equal(
      failed.state.notice?.title,
      "Somebody changed this reservation after you opened this page.",
    );
  });

  test("a cancellation that comes too late says so", () => {
    const failed = changeFailed(problem("cancellation-too-late", 422), "cancel");
    assert.equal(failed.stale, false);
    assert.equal(failed.state.notice?.title, "It is too late to cancel this reservation.");
  });

  test("a reservation that was cancelled meanwhile is shown again", () => {
    const failed = changeFailed(problem("reservation-cancelled", 409), "cancel");
    assert.equal(failed.stale, true);
    assert.equal(failed.state.notice?.title, "This reservation is cancelled.");
  });

  test("a refused rescheduling keeps what the person entered", () => {
    const values = { date: "2026-11-20", start: "10:00", end: "11:30", attendees: "40" };
    const failed = changeFailed(problem("over-capacity", 422), "reschedule", values);
    assert.equal(failed.stale, false);
    assert.deepEqual(failed.state.values, values);
    assert.equal(failed.state.fieldErrors.attendees, "More people attend than the room holds.");
  });
});

test("isVersion accepts a strong entity tag of digits", () => {
  assert.equal(isVersion('"1"'), true);
  assert.equal(isVersion('"12"'), true);
  assert.equal(isVersion("1"), false);
  assert.equal(isVersion('W/"1"'), false);
  assert.equal(isVersion(undefined), false);
});
