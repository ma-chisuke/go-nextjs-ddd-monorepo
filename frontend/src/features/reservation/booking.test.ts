import assert from "node:assert/strict";
import { describe, test } from "node:test";

import type { Problem } from "../../lib/problem.ts";
import {
  bookingFailed,
  bookingFingerprint,
  chooseKey,
  isIdempotencyKey,
  needsNewKey,
  readBooking,
  type NewReservation,
} from "./booking.ts";

const roomId = "01a1eeef-dde4-79b2-b195-89894d81fb14";

const filledIn = {
  roomId,
  date: "2026-11-20",
  timeZone: "Asia/Tokyo",
  idempotencyKey: "key-1",
  sentWith: "",
  start: "10:00",
  end: "11:30",
  attendees: "4",
  organizerEmail: " organizer@example.com ",
};

function form(fields: Record<string, string>): FormData {
  const data = new FormData();
  for (const [name, value] of Object.entries(fields)) {
    data.set(name, value);
  }
  return data;
}

// Makes the keys "new-1", "new-2", ... in order.
function keys(): () => string {
  let made = 0;
  return () => `new-${String(++made)}`;
}

const request: NewReservation = {
  roomId,
  startsAt: "2026-11-20T01:00:00.000Z",
  endsAt: "2026-11-20T02:30:00.000Z",
  attendees: 4,
  organizerEmail: "organizer@example.com",
};

describe("readBooking", () => {
  test("a filled-in form asks for the instants of its times in the zone of the room", () => {
    const read = readBooking(form(filledIn), keys());
    assert.equal(read.ok, true);
    assert.deepEqual(read.booking.request, request);
    assert.equal(read.booking.idempotencyKey, "key-1");
    assert.equal(read.booking.fingerprint, bookingFingerprint(request));
  });

  test("each field that does not fit gets a message, and the key stays", () => {
    const read = readBooking(
      form({ ...filledIn, start: "", end: "25:00", attendees: "0", organizerEmail: "nobody" }),
      keys(),
    );
    assert.equal(read.ok, false);
    assert.deepEqual(Object.keys(read.state.fieldErrors).sort(), [
      "attendees",
      "end",
      "organizerEmail",
      "start",
    ]);
    assert.equal(read.state.idempotencyKey, "key-1");
    assert.equal(read.state.values?.end, "25:00");
  });

  test("a time that the clocks skip is refused before the API sees it", () => {
    const read = readBooking(
      form({ ...filledIn, timeZone: "America/New_York", date: "2026-03-08", start: "02:30" }),
      keys(),
    );
    assert.equal(read.ok, false);
    assert.match(read.state.fieldErrors.start ?? "", /There is no 02:30/);
  });

  test("hidden fields that do not fit make the whole form unreadable", () => {
    for (const broken of [
      { roomId: "1" },
      { date: "2026-02-30" },
      { timeZone: "Mars/Olympus_Mons" },
    ]) {
      const read = readBooking(form({ ...filledIn, ...broken }), keys());
      assert.equal(read.ok, false);
      assert.equal(read.state.notice?.title, "This form cannot be read.");
    }
  });

  test("a form without a key that the API accepts gets a new one", () => {
    const read = readBooking(form({ ...filledIn, idempotencyKey: "has space" }), keys());
    assert.equal(read.ok, true);
    assert.equal(read.booking.idempotencyKey, "new-1");
  });
});

describe("the Idempotency-Key", () => {
  const fingerprint = bookingFingerprint(request);

  test("the first submit of a form sends the key of the form", () => {
    assert.equal(chooseKey({ key: "key-1" }, fingerprint, keys()), "key-1");
  });

  test("the same booking sent again sends the same key", () => {
    assert.equal(chooseKey({ key: "key-1", sentWith: fingerprint }, fingerprint, keys()), "key-1");
  });

  test("another booking from the same form gets a new key", () => {
    const changed = bookingFingerprint({ ...request, attendees: 5 });
    assert.notEqual(changed, fingerprint);
    assert.equal(chooseKey({ key: "key-1", sentWith: fingerprint }, changed, keys()), "new-1");
  });

  test("a submit after a changed booking keeps the key through the form", () => {
    // The person sent a booking that got no answer, changed the attendees,
    // and sent again: the form now carries the new key and the new booking.
    const changed = { ...request, attendees: 5 };
    const read = readBooking(form({ ...filledIn, attendees: "5", sentWith: fingerprint }), keys());
    assert.equal(read.ok, true);
    assert.equal(read.booking.idempotencyKey, "new-1");
    const failed = bookingFailed(
      { type: "about:blank", title: "Reservation does not answer", status: 503 },
      read.booking,
    );
    assert.equal(failed.sentWith, bookingFingerprint(changed));
    const again = readBooking(
      form({
        ...filledIn,
        attendees: "5",
        idempotencyKey: failed.idempotencyKey,
        sentWith: failed.sentWith ?? "",
      }),
      keys(),
    );
    assert.equal(again.ok, true);
    assert.equal(again.booking.idempotencyKey, "new-1");
  });

  test("a form that failed before anything went out keeps the booking of its key", () => {
    const read = readBooking(form({ ...filledIn, sentWith: fingerprint, attendees: "x" }), keys());
    assert.equal(read.ok, false);
    assert.equal(read.state.idempotencyKey, "key-1");
    assert.equal(read.state.sentWith, fingerprint);
  });

  test("the fingerprint compares instants, not how they are written", () => {
    assert.equal(
      bookingFingerprint({ ...request, startsAt: "2026-11-20T10:00:00+09:00" }),
      fingerprint,
    );
    assert.notEqual(
      bookingFingerprint({ ...request, organizerEmail: "other@example.com" }),
      fingerprint,
    );
  });

  test("a key that booked something else needs a new key", () => {
    const reused: Problem = {
      type: "https://example.com/docs/problems/idempotency-key-reused.md",
      title: "The idempotency key came with another request",
      status: 422,
    };
    const reservation = {
      id: "01a1eeef-dde4-79b2-b195-89894d81fb15",
      roomId,
      startsAt: request.startsAt,
      endsAt: request.endsAt,
      attendees: 4,
      organizerEmail: "organizer@example.com",
    };
    assert.equal(needsNewKey({ ok: false, problem: reused }), true);
    assert.equal(needsNewKey({ ok: true, value: { ...reservation, status: "cancelled" } }), true);
    assert.equal(needsNewKey({ ok: true, value: { ...reservation, status: "confirmed" } }), false);
    assert.equal(needsNewKey({ ok: false, problem: { ...reused, type: "about:blank" } }), false);
  });

  test("a key is 1 to 255 visible ASCII characters", () => {
    assert.equal(isIdempotencyKey("2f0c3a4e-6b1d-4a43-9d0e-0d5a3c1f7b21"), true);
    assert.equal(isIdempotencyKey(""), false);
    assert.equal(isIdempotencyKey("a b"), false);
    assert.equal(isIdempotencyKey("x".repeat(256)), false);
  });
});

test("a refused booking shows the words of its kind and keeps the key", () => {
  const read = readBooking(form(filledIn), keys());
  assert.equal(read.ok, true);
  const state = bookingFailed(
    {
      type: "https://example.com/docs/problems/room-already-booked.md",
      title: "The room is booked for the period",
      status: 409,
    },
    read.booking,
  );
  assert.equal(state.notice?.title, "The room is already booked for part of this period.");
  assert.deepEqual(state.noticeFields, ["start", "end"]);
  assert.equal(state.idempotencyKey, "key-1");
  assert.equal(state.values?.organizerEmail, "organizer@example.com");
});
