// The period of a reservation as a person reads and enters it: a date and
// times of day in the time zone of the room. The API takes and gives the
// instants.

import type { FieldErrors } from "../../lib/input.ts";
import { formatDate, instantsAt, isTimeOfDay, wallTime } from "../../lib/zoned-time.ts";

/**
 * The instants of a period that a person entered as a date and two times of
 * day in the time zone of the room, or the messages for the times that stand
 * for no instant. `date` is a date and `timeZone` a zone that JavaScript knows.
 *
 * Where the clocks move forward, the times that they skip do not exist, and
 * the person is told so. Where they move back, the times in between happen
 * twice: the start is read as the first of the two, and the end as the first
 * one after the start, which is how a person reads "01:00 to 01:30" that
 * night.
 */
export function readPeriod(
  date: string,
  start: string,
  end: string,
  timeZone: string,
):
  | { ok: true; startsAt: Date; endsAt: Date }
  | { ok: false; fieldErrors: FieldErrors<"start" | "end"> } {
  const at = (time: string, after?: Date): Date | string => {
    if (!isTimeOfDay(time)) {
      return "Enter a time of day on a 24-hour clock, such as 10:00.";
    }
    const local = instantsAt(date, time, timeZone);
    switch (local.kind) {
      case "unique":
        return local.instant;
      case "repeated":
        return after === undefined || local.earlier.getTime() > after.getTime()
          ? local.earlier
          : local.later;
      case "skipped":
        return `There is no ${time} on ${formatDate(date)} in ${timeZone}: the clocks skip it.`;
    }
  };
  const startsAt = at(start);
  const endsAt = at(end, startsAt instanceof Date ? startsAt : undefined);
  if (startsAt instanceof Date && endsAt instanceof Date) {
    return { ok: true, startsAt, endsAt };
  }
  const fieldErrors: FieldErrors<"start" | "end"> = {};
  if (typeof startsAt === "string") {
    fieldErrors.start = startsAt;
  }
  if (typeof endsAt === "string") {
    fieldErrors.end = endsAt;
  }
  return { ok: false, fieldErrors };
}

/** The period of a reservation on the clocks of a time zone. */
export interface LocalPeriod {
  /** The date on which the period starts, such as "2026-11-20". */
  date: string;
  /** The time of day at which it starts, such as "10:00". */
  start: string;
  /** The date on which it ends; the same as `date` but for a period that runs past midnight. */
  endDate: string;
  /** The time of day at which it ends, such as "11:00". */
  end: string;
}

/** The period from `startsAt` to `endsAt` (RFC 3339) on the clocks of a time zone. */
export function localPeriod(startsAt: string, endsAt: string, timeZone: string): LocalPeriod {
  const start = wallTime(new Date(startsAt), timeZone);
  const end = wallTime(new Date(endsAt), timeZone);
  return { date: start.date, start: start.time, endDate: end.date, end: end.time };
}

/**
 * The times of a period, such as "10:00 to 11:00". A date is written only
 * where it differs from `day`, the date that the screen shows.
 */
export function formatTimes(period: LocalPeriod, day: string): string {
  const start = period.date === day ? period.start : `${formatDate(period.date)}, ${period.start}`;
  const end =
    period.endDate === period.date ? period.end : `${formatDate(period.endDate)}, ${period.end}`;
  return `${start} to ${end}`;
}

/** The whole period, such as "Friday 20 November 2026, 10:00 to 11:00". */
export function formatPeriod(period: LocalPeriod): string {
  return `${formatDate(period.date)}, ${formatTimes(period, period.date)}`;
}
