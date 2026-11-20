import assert from "node:assert/strict";
import { describe, test } from "node:test";

import {
  addDays,
  dateIn,
  dayWindow,
  formatDate,
  instantsAt,
  isDate,
  isTimeOfDay,
  isTimeZone,
  startOfDay,
  timeZoneNames,
  wallTime,
  type LocalTime,
} from "./zoned-time.ts";

const hourMs = 3_600_000;

// The expected instants come from Temporal in Node, which knows the same
// time zone database.
function unique(localTime: LocalTime): string {
  assert.equal(localTime.kind, "unique");
  return localTime.instant.toISOString();
}

describe("UTC", () => {
  test("a time of day is the instant of the same reading", () => {
    assert.equal(unique(instantsAt("2026-11-20", "10:00", "UTC")), "2026-11-20T10:00:00.000Z");
    assert.deepEqual(wallTime(new Date("2026-11-20T10:00:00Z"), "UTC"), {
      date: "2026-11-20",
      time: "10:00",
    });
  });

  test("a day lasts 24 hours", () => {
    const { from, to } = dayWindow("2026-11-20", "UTC");
    assert.equal(from.toISOString(), "2026-11-20T00:00:00.000Z");
    assert.equal(to.toISOString(), "2026-11-21T00:00:00.000Z");
  });
});

describe("Asia/Tokyo, which has no daylight saving time", () => {
  test("a time of day is nine hours ahead of UTC", () => {
    assert.equal(
      unique(instantsAt("2026-11-20", "10:00", "Asia/Tokyo")),
      "2026-11-20T01:00:00.000Z",
    );
    assert.deepEqual(wallTime(new Date("2026-11-20T01:00:00Z"), "Asia/Tokyo"), {
      date: "2026-11-20",
      time: "10:00",
    });
  });

  test("an instant late in the UTC day falls on the next date", () => {
    assert.deepEqual(wallTime(new Date("2026-11-19T15:30:00Z"), "Asia/Tokyo"), {
      date: "2026-11-20",
      time: "00:30",
    });
    assert.equal(dateIn("Asia/Tokyo", new Date("2026-11-19T15:30:00Z")), "2026-11-20");
    assert.equal(dateIn("UTC", new Date("2026-11-19T15:30:00Z")), "2026-11-19");
  });

  test("a day starts at 15:00 UTC the day before", () => {
    const { from, to } = dayWindow("2026-11-20", "Asia/Tokyo");
    assert.equal(from.toISOString(), "2026-11-19T15:00:00.000Z");
    assert.equal(to.getTime() - from.getTime(), 24 * hourMs);
  });
});

describe("America/New_York, which has daylight saving time", () => {
  test("on the day the clocks move forward, the times from 02:00 to 02:59 do not exist", () => {
    for (const time of ["02:00", "02:30", "02:59"]) {
      const localTime = instantsAt("2026-03-08", time, "America/New_York");
      assert.equal(localTime.kind, "skipped", time);
      // At 02:00 EST the clocks jump to 03:00 EDT.
      assert.equal(localTime.next.toISOString(), "2026-03-08T07:00:00.000Z", time);
    }
  });

  test("on the day the clocks move forward, the times around the gap exist once", () => {
    assert.equal(
      unique(instantsAt("2026-03-08", "01:59", "America/New_York")),
      "2026-03-08T06:59:00.000Z",
    );
    assert.equal(
      unique(instantsAt("2026-03-08", "03:00", "America/New_York")),
      "2026-03-08T07:00:00.000Z",
    );
  });

  test("the day the clocks move forward lasts 23 hours", () => {
    const { from, to } = dayWindow("2026-03-08", "America/New_York");
    assert.equal(from.toISOString(), "2026-03-08T05:00:00.000Z");
    assert.equal(to.getTime() - from.getTime(), 23 * hourMs);
  });

  test("on the day the clocks move back, 01:30 happens twice", () => {
    const localTime = instantsAt("2026-11-01", "01:30", "America/New_York");
    assert.equal(localTime.kind, "repeated");
    assert.equal(localTime.earlier.toISOString(), "2026-11-01T05:30:00.000Z");
    assert.equal(localTime.later.toISOString(), "2026-11-01T06:30:00.000Z");
    assert.deepEqual(wallTime(localTime.earlier, "America/New_York"), {
      date: "2026-11-01",
      time: "01:30",
    });
    assert.deepEqual(wallTime(localTime.later, "America/New_York"), {
      date: "2026-11-01",
      time: "01:30",
    });
  });

  test("the day the clocks move back lasts 25 hours", () => {
    const { from, to } = dayWindow("2026-11-01", "America/New_York");
    assert.equal(from.toISOString(), "2026-11-01T04:00:00.000Z");
    assert.equal(to.getTime() - from.getTime(), 25 * hourMs);
  });

  test("a time of day reads back as itself, before and after the change", () => {
    for (const date of ["2026-03-07", "2026-03-09", "2026-10-31", "2026-11-02"]) {
      for (const time of ["00:00", "09:15", "12:30", "23:45"]) {
        const instant = instantsAt(date, time, "America/New_York");
        assert.equal(instant.kind, "unique", `${date} ${time}`);
        assert.deepEqual(wallTime(instant.instant, "America/New_York"), { date, time });
      }
    }
  });
});

describe("Europe/Berlin, east of Greenwich, which has daylight saving time", () => {
  test("on the day the clocks move forward, 02:30 does not exist", () => {
    const localTime = instantsAt("2026-03-29", "02:30", "Europe/Berlin");
    assert.equal(localTime.kind, "skipped");
    // At 02:00 CET the clocks jump to 03:00 CEST.
    assert.equal(localTime.next.toISOString(), "2026-03-29T01:00:00.000Z");
    assert.equal(
      unique(instantsAt("2026-03-29", "01:59", "Europe/Berlin")),
      "2026-03-29T00:59:00.000Z",
    );
  });

  test("on the day the clocks move back, 02:30 happens twice", () => {
    const localTime = instantsAt("2026-10-25", "02:30", "Europe/Berlin");
    assert.equal(localTime.kind, "repeated");
    assert.equal(localTime.earlier.toISOString(), "2026-10-25T00:30:00.000Z");
    assert.equal(localTime.later.toISOString(), "2026-10-25T01:30:00.000Z");
  });
});

describe("America/Havana, where the clocks move back to midnight", () => {
  test("a day whose midnight happens twice starts at the first one", () => {
    // At 01:00 CDT the clocks go back to 00:00 CST.
    const { from, to } = dayWindow("2026-11-01", "America/Havana");
    assert.equal(from.toISOString(), "2026-11-01T04:00:00.000Z");
    assert.equal(to.getTime() - from.getTime(), 25 * hourMs);
  });
});

describe("America/Santiago, where the clocks jump over midnight", () => {
  test("a day whose midnight does not exist starts when the clocks land", () => {
    // At 00:00 -04 the clocks jump to 01:00 -03.
    assert.equal(instantsAt("2026-09-06", "00:00", "America/Santiago").kind, "skipped");
    assert.equal(
      startOfDay("2026-09-06", "America/Santiago").toISOString(),
      "2026-09-06T04:00:00.000Z",
    );
    const { from, to } = dayWindow("2026-09-06", "America/Santiago");
    assert.equal(to.getTime() - from.getTime(), 23 * hourMs);
  });

  test("when the clocks move back to 23:00, the last hour of the day before happens twice", () => {
    const localTime = instantsAt("2026-04-04", "23:30", "America/Santiago");
    assert.equal(localTime.kind, "repeated");
    assert.equal(localTime.earlier.toISOString(), "2026-04-05T02:30:00.000Z");
    assert.equal(localTime.later.toISOString(), "2026-04-05T03:30:00.000Z");
    assert.equal(
      startOfDay("2026-04-05", "America/Santiago").toISOString(),
      "2026-04-05T04:00:00.000Z",
    );
  });
});

test("isDate accepts dates of the calendar only", () => {
  assert.equal(isDate("2026-11-20"), true);
  assert.equal(isDate("2028-02-29"), true);
  assert.equal(isDate("2026-02-29"), false);
  assert.equal(isDate("2026-13-01"), false);
  assert.equal(isDate("2026-1-1"), false);
  assert.equal(isDate("20 November 2026"), false);
  assert.equal(isDate("x2026-11-20"), false);
  assert.equal(isDate("2026-11-20T10:00"), false);
  // JavaScript reads a year before 100 as one of 1900 to 1999.
  assert.equal(isDate("0099-12-31"), false);
  assert.equal(isDate(""), false);
});

test("isTimeOfDay accepts hours and minutes on a 24-hour clock", () => {
  assert.equal(isTimeOfDay("00:00"), true);
  assert.equal(isTimeOfDay("23:59"), true);
  assert.equal(isTimeOfDay("24:00"), false);
  assert.equal(isTimeOfDay("9:30"), false);
  assert.equal(isTimeOfDay("T09:30"), false);
  assert.equal(isTimeOfDay("09:30:00"), false);
});

test("isTimeZone knows the zones of the IANA database", () => {
  assert.equal(isTimeZone("Asia/Tokyo"), true);
  assert.equal(isTimeZone("UTC"), true);
  assert.equal(isTimeZone("Mars/Olympus_Mons"), false);
  assert.equal(isTimeZone(""), false);
});

test("timeZoneNames lists UTC first, once, and the zones of the IANA database", () => {
  const names = timeZoneNames();
  assert.equal(names[0], "UTC");
  assert.equal(names.filter((name) => name === "UTC").length, 1);
  assert.ok(names.includes("Asia/Tokyo"));
  assert.ok(names.includes("America/New_York"));
  assert.ok(names.every(isTimeZone));
});

test("addDays crosses months and years", () => {
  assert.equal(addDays("2026-11-30", 1), "2026-12-01");
  assert.equal(addDays("2026-12-31", 1), "2027-01-01");
  assert.equal(addDays("2026-03-01", -1), "2026-02-28");
  assert.equal(addDays("2026-03-08", 1), "2026-03-09");
});

test("formatDate writes the weekday, the day, the month, and the year", () => {
  assert.equal(formatDate("2026-11-20"), "Friday 20 November 2026");
  assert.equal(formatDate("2027-01-03"), "Sunday 3 January 2027");
});

test("formatDate names every month and every weekday", () => {
  // The first days of the months of 2026 fall on each of the seven weekdays.
  for (const [date, text] of [
    ["2026-01-01", "Thursday 1 January 2026"],
    ["2026-02-01", "Sunday 1 February 2026"],
    ["2026-03-01", "Sunday 1 March 2026"],
    ["2026-04-01", "Wednesday 1 April 2026"],
    ["2026-05-01", "Friday 1 May 2026"],
    ["2026-06-01", "Monday 1 June 2026"],
    ["2026-07-01", "Wednesday 1 July 2026"],
    ["2026-08-01", "Saturday 1 August 2026"],
    ["2026-09-01", "Tuesday 1 September 2026"],
    ["2026-10-01", "Thursday 1 October 2026"],
    ["2026-11-01", "Sunday 1 November 2026"],
    ["2026-12-01", "Tuesday 1 December 2026"],
  ] as const) {
    assert.equal(formatDate(date), text);
  }
});

test("a value that is not a date or a time of day is refused", () => {
  assert.throws(() => instantsAt("2026-02-30", "10:00", "UTC"), {
    name: "RangeError",
    message: "2026-02-30 is not a date such as 2026-11-20",
  });
  assert.throws(() => instantsAt("2026-11-20", "25:00", "UTC"), {
    name: "RangeError",
    message: "25:00 is not a time of day such as 09:30",
  });
  assert.throws(() => instantsAt("2026-11-20", "10:00", "Mars/Olympus_Mons"), RangeError);
});
