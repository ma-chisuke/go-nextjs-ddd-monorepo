// Values that come from outside, such as the fields of a form and the
// parameters of a URL. A Server Function is a public entry point that anyone
// can post to, so it narrows every value with these before it uses one, and
// it answers a value that does not fit with a message for the form.

import { fieldErrors, type Problem } from "./problem.ts";

/** A message about a whole form, in the words of the screen. */
export interface Notice {
  title: string;
  detail?: string;
}

/** The messages for the fields of a form that are not valid, by the name of the field. */
export type FieldErrors<Field extends string> = Partial<Record<Field, string>>;

/**
 * The text of a field of a form, without the spaces around it. A field that
 * is missing, or that holds a file, has no text: "".
 */
export function formText(form: FormData, name: string): string {
  const value = form.get(name);
  return typeof value === "string" ? value.trim() : "";
}

/** A whole number from `min` to `max`, written in decimal digits, or undefined. */
export function wholeNumber(value: string, min: number, max: number): number | undefined {
  if (!/^\d{1,9}$/.test(value)) {
    return undefined;
  }
  const number = Number(value);
  return number >= min && number <= max ? number : undefined;
}

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Whether a value is a UUID, the form of every id of the APIs. */
export function isUuid(value: string): boolean {
  return uuidPattern.test(value);
}

// The pattern and the longest email address of the API documents. The
// pattern needs at least three characters, which is the shortest length that
// the documents allow. The documents also refuse U+0000, which the API
// cannot store; it is checked apart, because a pattern with a control
// character is hard to read.
const emailPattern = /^[^@\s]+@[^@\s]+$/;

/** Whether a value is an email address as the API documents accept one. */
export function isEmailAddress(value: string): boolean {
  return value.length <= 254 && !value.includes("\u0000") && emailPattern.test(value);
}

/**
 * The messages of a problem for the fields of a form. `fieldOf` names the
 * field of the form for each part of the request that the problem points at;
 * a part that no field stands for is left out.
 */
export function problemFieldErrors<Field extends string>(
  problem: Problem,
  fieldOf: Partial<Record<string, Field>>,
): FieldErrors<Field> {
  const messages: FieldErrors<Field> = {};
  for (const [part, message] of Object.entries(fieldErrors(problem))) {
    const field = fieldOf[part];
    if (field !== undefined && messages[field] === undefined) {
      messages[field] = message;
    }
  }
  return messages;
}

/** The problem as it is: its title, and what went wrong this time. */
export function problemNotice(problem: Problem): Notice {
  return problem.detail === undefined
    ? { title: problem.title }
    : { title: problem.title, detail: problem.detail };
}
