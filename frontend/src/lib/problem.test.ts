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
  // Only the extension .md comes off the name of the page.
  assert.equal(
    problemKind({ type: "/docs/problems/notes.mdx", title: "", status: 500 }),
    "notes.mdx",
  );
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
  assert.deepEqual(unavailable("Facility", new Error("fetch failed")), {
    type: "about:blank",
    title: "Facility does not answer",
    status: 503,
    detail: "fetch failed",
  });
  assert.equal(unavailable("Facility", "timeout").detail, "The service could not be reached.");
});

test("isProblem tells a problem from another answer", () => {
  for (const [value, expected] of [
    [invalid, true],
    [{ type: "about:blank", title: "x", status: 502 }, true],
    [{ title: "x", status: 502 }, false],
    [{ type: "about:blank", status: 502 }, false],
    [{ type: "about:blank", title: "x" }, false],
    [{ type: "about:blank", title: "x", status: "502" }, false],
    ["Bad Gateway", false],
    [null, false],
    [undefined, false],
  ] as const) {
    assert.equal(isProblem(value), expected, JSON.stringify(value));
  }
});
