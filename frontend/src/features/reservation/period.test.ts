import assert from "node:assert/strict";
import { test } from "node:test";

import { formatPeriod, formatTimes, localPeriod, readPeriod } from "./period.ts";

test("a period entered in Asia/Tokyo is nine hours ahead of UTC", () => {
  assert.deepEqual(readPeriod("2026-11-20", "10:00", "11:30", "Asia/Tokyo"), {
    ok: true,
    startsAt: new Date("2026-11-20T01:00:00Z"),
    endsAt: new Date("2026-11-20T02:30:00Z"),
  });
});

test("a period entered in UTC is the same reading", () => {
  assert.deepEqual(readPeriod("2026-11-20", "10:00", "11:30", "UTC"), {
    ok: true,
    startsAt: new Date("2026-11-20T10:00:00Z"),
    endsAt: new Date("2026-11-20T11:30:00Z"),
  });
});

test("a time that the clocks skip is refused at its field", () => {
  const read = readPeriod("2026-03-08", "01:30", "02:30", "America/New_York");
  assert.deepEqual(read, {
    ok: false,
    fieldErrors: {
      end: "There is no 02:30 on Sunday 8 March 2026 in America/New_York: the clocks skip it.",
    },
  });
});

test("across the gap, a period is an hour shorter than its times of day say", () => {
  const read = readPeriod("2026-03-08", "01:30", "03:30", "America/New_York");
  assert.equal(read.ok, true);
  assert.equal(read.endsAt.getTime() - read.startsAt.getTime(), 3_600_000);
});

test("a time that happens twice starts at its first instant", () => {
  assert.deepEqual(readPeriod("2026-11-01", "01:00", "01:30", "America/New_York"), {
    ok: true,
    startsAt: new Date("2026-11-01T05:00:00Z"),
    endsAt: new Date("2026-11-01T05:30:00Z"),
  });
});

test("an end that happens twice is the first one after the start", () => {
  // 01:45 EDT to 01:15 EST, after the clocks went back from 02:00 to 01:00.
  assert.deepEqual(readPeriod("2026-11-01", "01:45", "01:15", "America/New_York"), {
    ok: true,
    startsAt: new Date("2026-11-01T05:45:00Z"),
    endsAt: new Date("2026-11-01T06:15:00Z"),
  });
});

test("times that are not times of day are refused at their fields", () => {
  const read = readPeriod("2026-11-20", "", "10", "UTC");
  assert.equal(read.ok, false);
  assert.deepEqual(Object.keys(read.fieldErrors), ["start", "end"]);
});

test("a period reads in the time zone of the room", () => {
  const period = localPeriod("2026-11-20T01:00:00Z", "2026-11-20T02:30:00Z", "Asia/Tokyo");
  assert.deepEqual(period, {
    date: "2026-11-20",
    start: "10:00",
    endDate: "2026-11-20",
    end: "11:30",
  });
  assert.equal(formatTimes(period, "2026-11-20"), "10:00 to 11:30");
  assert.equal(formatPeriod(period), "Friday 20 November 2026, 10:00 to 11:30");
});

test("the same instants read differently in another zone", () => {
  const period = localPeriod("2026-11-20T01:00:00Z", "2026-11-20T02:30:00Z", "UTC");
  assert.deepEqual(period, {
    date: "2026-11-20",
    start: "01:00",
    endDate: "2026-11-20",
    end: "02:30",
  });
});

test("a period across the change of the clocks keeps its times of day", () => {
  // 01:00 EDT to 01:30 EST: the clocks go back at 02:00 EDT.
  const period = localPeriod("2026-11-01T05:00:00Z", "2026-11-01T06:30:00Z", "America/New_York");
  assert.equal(formatTimes(period, "2026-11-01"), "01:00 to 01:30");
});

test("a date is written where it is not the day on the screen", () => {
  const period = localPeriod("2026-11-20T14:00:00Z", "2026-11-20T15:30:00Z", "Asia/Tokyo");
  assert.equal(formatTimes(period, "2026-11-20"), "23:00 to Saturday 21 November 2026, 00:30");
  assert.equal(
    formatTimes(period, "2026-11-21"),
    "Friday 20 November 2026, 23:00 to Saturday 21 November 2026, 00:30",
  );
});
