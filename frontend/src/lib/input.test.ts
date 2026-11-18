import assert from "node:assert/strict";
import { test } from "node:test";

import {
  formText,
  isEmailAddress,
  isUuid,
  problemFieldErrors,
  problemNotice,
  wholeNumber,
} from "./input.ts";

test("formText gives the text of a field without the spaces around it", () => {
  const form = new FormData();
  form.set("name", "  Aoba ");
  form.set("file", new Blob(["x"]), "x.txt");
  assert.equal(formText(form, "name"), "Aoba");
  assert.equal(formText(form, "missing"), undefined);
  assert.equal(formText(form, "file"), undefined);
});

test("wholeNumber accepts decimal digits inside the limits", () => {
  assert.equal(wholeNumber("8", 1, 1000), 8);
  assert.equal(wholeNumber("1000", 1, 1000), 1000);
  assert.equal(wholeNumber("0", 1, 1000), undefined);
  assert.equal(wholeNumber("1001", 1, 1000), undefined);
  assert.equal(wholeNumber("2.5", 1, 1000), undefined);
  assert.equal(wholeNumber("1e2", 1, 1000), undefined);
  assert.equal(wholeNumber("-3", 1, 1000), undefined);
  assert.equal(wholeNumber("", 1, 1000), undefined);
  assert.equal(wholeNumber(undefined, 1, 1000), undefined);
});

test("isUuid accepts the ids of the APIs only", () => {
  assert.equal(isUuid("01a1eeef-dde4-79b2-b195-89894d81fb14"), true);
  assert.equal(isUuid("01A1EEEF-DDE4-79B2-B195-89894D81FB14"), true);
  assert.equal(isUuid("01a1eeef-dde4-79b2-b195-89894d81fb1"), false);
  assert.equal(isUuid("../rooms"), false);
  assert.equal(isUuid(undefined), false);
});

test("isEmailAddress follows the pattern of the API documents", () => {
  assert.equal(isEmailAddress("organizer@example.com"), true);
  assert.equal(isEmailAddress("a@b"), true);
  assert.equal(isEmailAddress("organizer"), false);
  assert.equal(isEmailAddress("two@at@example.com"), false);
  assert.equal(isEmailAddress("with space@example.com"), false);
  assert.equal(isEmailAddress(`${"a".repeat(250)}@b.cd`), false);
  assert.equal(isEmailAddress(undefined), false);
});

test("problemFieldErrors names the fields of the form that a problem points at", () => {
  const problem = {
    type: "/docs/problems/validation-error.md",
    title: "The request is not valid",
    status: 422,
    errors: [
      { pointer: "/startsAt", detail: "must be a date-time" },
      { pointer: "/roomId", detail: "must be a uuid" },
      { pointer: "/attendees", detail: "must be at least 1" },
    ],
  };
  assert.deepEqual(problemFieldErrors(problem, { startsAt: "start", attendees: "attendees" }), {
    start: "must be a date-time",
    attendees: "must be at least 1",
  });
});

test("problemNotice shows the problem as it is", () => {
  assert.deepEqual(problemNotice({ type: "about:blank", title: "Conflict", status: 409 }), {
    title: "Conflict",
  });
  assert.deepEqual(
    problemNotice({ type: "about:blank", title: "Conflict", status: 409, detail: "Why" }),
    { title: "Conflict", detail: "Why" },
  );
});
