// Dates and times of day in a time zone, and the instants that they stand
// for. The APIs take and give instants (RFC 3339); a person reads and enters
// a date and a time of day in the time zone of the room. This module converts
// between the two.
//
// It uses Intl.DateTimeFormat, the one part of JavaScript that knows the IANA
// time zones everywhere. Temporal would do the same in fewer lines, and the
// Node of mise.toml has it, but Safari does not ship it yet (2026-11), and
// these functions must stay usable in the browser. No date library.
//
// A date is "2026-11-20", and a time of day is "09:30", on a 24-hour clock,
// the forms in which <input type="date"> and <input type="time"> give them.

const minuteMs = 60_000;
const dayMs = 24 * 60 * minuteMs;

const datePattern = /^(\d{4})-(\d{2})-(\d{2})$/;
const timePattern = /^([01]\d|2[0-3]):([0-5]\d)$/;

/** Whether a value is a date of the calendar, such as "2026-11-20". */
export function isDate(value: string): boolean {
  const parts = dateParts(value);
  return parts !== undefined;
}

/** Whether a value is a time of day in hours and minutes, such as "09:30". */
export function isTimeOfDay(value: string): boolean {
  return timePattern.test(value);
}

/** Whether JavaScript knows a value as a time zone, such as "Asia/Tokyo". */
export function isTimeZone(value: string): boolean {
  if (value === "") {
    return false;
  }
  try {
    formatter(value);
    return true;
  } catch {
    return false;
  }
}

/** The date that comes a number of days after a date, or before it when the number is negative. */
export function addDays(date: string, days: number): string {
  const parts = requireDate(date);
  const moved = new Date(Date.UTC(parts.year, parts.month - 1, parts.day) + days * dayMs);
  return formatDateParts(moved.getUTCFullYear(), moved.getUTCMonth() + 1, moved.getUTCDate());
}

/** What the clock and the calendar show in a time zone at an instant. */
export function wallTime(instant: Date, timeZone: string): { date: string; time: string } {
  const wall = new Date(wallMs(instant.getTime(), timeZone));
  return {
    date: formatDateParts(wall.getUTCFullYear(), wall.getUTCMonth() + 1, wall.getUTCDate()),
    time: `${pad(wall.getUTCHours(), 2)}:${pad(wall.getUTCMinutes(), 2)}`,
  };
}

/** The date in a time zone at an instant, such as now. */
export function dateIn(timeZone: string, instant: Date): string {
  return wallTime(instant, timeZone).date;
}

/**
 * The instants at which the clocks of a time zone show a date and a time of
 * day. Usually there is one. When the clocks move back, the times in between
 * happen twice; when they move forward, the times in between never happen.
 */
export type LocalTime =
  | { kind: "unique"; instant: Date }
  /** The time happens twice: first at `earlier`, and again at `later`. */
  | { kind: "repeated"; earlier: Date; later: Date }
  /** The time does not exist: the clocks jumped over it at `next`, the first instant after the gap. */
  | { kind: "skipped"; next: Date };

/** The instants of a date and a time of day in a time zone. */
export function instantsAt(date: string, time: string, timeZone: string): LocalTime {
  const local = localMs(date, time);
  // The offset of the zone a day before and a day after. A transition of the
  // clocks between the two is what makes a time repeat or vanish.
  const before = offsetAt(local - dayMs, timeZone);
  const after = offsetAt(local + dayMs, timeZone);
  const candidates = [...new Set([local - before, local - after])]
    .filter((instant) => instant + offsetAt(instant, timeZone) === local)
    .sort((a, b) => a - b);
  const [first, second] = candidates;
  if (first !== undefined && second !== undefined) {
    return { kind: "repeated", earlier: new Date(first), later: new Date(second) };
  }
  if (first !== undefined) {
    return { kind: "unique", instant: new Date(first) };
  }
  return { kind: "skipped", next: new Date(transition(local - after, local - before, timeZone)) };
}

/**
 * The first instant of a date in a time zone: midnight, or, where the clocks
 * jump over midnight, the instant at which they land on that date.
 */
export function startOfDay(date: string, timeZone: string): Date {
  const midnight = instantsAt(date, "00:00", timeZone);
  switch (midnight.kind) {
    case "unique":
      return midnight.instant;
    case "repeated":
      return midnight.earlier;
    case "skipped":
      return midnight.next;
  }
}

/** The instants from the start of a date in a time zone until the start of the next date. */
export function dayWindow(date: string, timeZone: string): { from: Date; to: Date } {
  return { from: startOfDay(date, timeZone), to: startOfDay(addDays(date, 1), timeZone) };
}

const weekdays = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const months = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

/**
 * A date as people read it, such as "Friday 20 November 2026". The words are
 * written out here, so that the server and every browser show the same text.
 */
export function formatDate(date: string): string {
  const parts = requireDate(date);
  const weekday = new Date(Date.UTC(parts.year, parts.month - 1, parts.day)).getUTCDay();
  return `${weekdays[weekday] ?? ""} ${String(parts.day)} ${months[parts.month - 1] ?? ""} ${String(parts.year)}`;
}

// The search for the instant at which the clocks jumped forward: the offset
// is the earlier one at `low` and the later one at `high`. One second is
// precise enough for any zone.
function transition(low: number, high: number, timeZone: string): number {
  const target = offsetAt(high, timeZone);
  let lo = low;
  let hi = high;
  while (hi - lo > 1000) {
    const middle = lo + Math.max(1, Math.floor((hi - lo) / 2000)) * 1000;
    if (offsetAt(middle, timeZone) === target) {
      hi = middle;
    } else {
      lo = middle;
    }
  }
  return hi;
}

// The difference between the clock of a zone and UTC at an instant, in
// milliseconds: positive east of Greenwich.
function offsetAt(instantMs: number, timeZone: string): number {
  const second = Math.floor(instantMs / 1000) * 1000;
  return wallMs(second, timeZone) - second;
}

// What the clock of a zone shows at an instant, as the milliseconds of the
// same reading in UTC.
function wallMs(instantMs: number, timeZone: string): number {
  const fields: Record<string, number> = {};
  for (const part of formatter(timeZone).formatToParts(new Date(instantMs))) {
    if (part.type !== "literal") {
      fields[part.type] = Number(part.value);
    }
  }
  return Date.UTC(
    fields.year ?? 0,
    (fields.month ?? 1) - 1,
    fields.day ?? 1,
    fields.hour ?? 0,
    fields.minute ?? 0,
    fields.second ?? 0,
  );
}

const formatters = new Map<string, Intl.DateTimeFormat>();

// One formatter per zone, made once: making one is slow. It throws a
// RangeError for a zone that JavaScript does not know, before it is kept.
function formatter(timeZone: string): Intl.DateTimeFormat {
  let found = formatters.get(timeZone);
  if (found === undefined) {
    found = new Intl.DateTimeFormat("en-US", {
      timeZone,
      hourCycle: "h23",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
    formatters.set(timeZone, found);
  }
  return found;
}

// A date and a time of day as the milliseconds of the same reading in UTC.
function localMs(date: string, time: string): number {
  const parts = requireDate(date);
  const match = timePattern.exec(time);
  if (!match) {
    throw new RangeError(`${time} is not a time of day such as 09:30`);
  }
  return Date.UTC(parts.year, parts.month - 1, parts.day, Number(match[1]), Number(match[2]));
}

interface DateParts {
  year: number;
  month: number;
  day: number;
}

function dateParts(value: string): DateParts | undefined {
  const match = datePattern.exec(value);
  if (!match) {
    return undefined;
  }
  const [year, month, day] = [Number(match[1]), Number(match[2]), Number(match[3])];
  const date = new Date(Date.UTC(year, month - 1, day));
  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month - 1 ||
    date.getUTCDate() !== day
  ) {
    return undefined;
  }
  return { year, month, day };
}

function requireDate(value: string): DateParts {
  const parts = dateParts(value);
  if (parts === undefined) {
    throw new RangeError(`${value} is not a date such as 2026-11-20`);
  }
  return parts;
}

function formatDateParts(year: number, month: number, day: number): string {
  return `${pad(year, 4)}-${pad(month, 2)}-${pad(day, 2)}`;
}

function pad(value: number, width: number): string {
  return String(value).padStart(width, "0");
}
