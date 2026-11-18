import assert from "node:assert/strict";
import { test } from "node:test";

import { fieldErrors, isProblem, problemKind, unavailable, type Problem } from "./problem.ts";

const invalid: Problem = {
  type: "https://github.com/ma-chisuke/go-nextjs-ddd-monorepo/blob/main/docs/problems/invalid-request.md",
  title: "The request is not valid",
  status: 400,
  errors: [
    { pointer: "/startsAt", detail: "must be a date-time" },
    { pointer: "/startsAt", detail: "a second message for the same field" },
    { pointer: "/attendees/0", detail: "must be at least 1" },
    { pointer: "limit", detail: "must be at most 100" },
    { pointer: "", detail: "the body must be an object" },
  ],
};

test("problemKind is the name of the page of the problem", () => {
  assert.equal(problemKind(invalid), "invalid-request");
  assert.equal(
    problemKind({ type: "/docs/problems/room-already-booked.md", title: "", status: 409 }),
    "room-already-booked",
  );
  assert.equal(problemKind({ type: "about:blank", title: "", status: 503 }), "about:blank");
});

test("fieldErrors gives the first message of each field", () => {
  assert.deepEqual(fieldErrors(invalid), {
    startsAt: "must be a date-time",
    attendees: "must be at least 1",
    limit: "must be at most 100",
  });
});

test("fieldErrors of a problem without invalid parts is empty", () => {
  assert.deepEqual(fieldErrors({ type: "about:blank", title: "Conflict", status: 409 }), {});
});

test("unavailable makes a problem of a failure that brought none", () => {
  const problem = unavailable("Facility", new Error("fetch failed"));
  assert.equal(problem.status, 503);
  assert.equal(problem.title, "Facility does not answer");
  assert.equal(problem.detail, "fetch failed");
  assert.equal(unavailable("Facility", "timeout").detail, "The service could not be reached.");
});

test("isProblem tells a problem from another answer", () => {
  assert.equal(isProblem(invalid), true);
  assert.equal(isProblem({ type: "about:blank", title: "x" }), false);
  assert.equal(isProblem("Bad Gateway"), false);
  assert.equal(isProblem(null), false);
  assert.equal(isProblem(undefined), false);
});
