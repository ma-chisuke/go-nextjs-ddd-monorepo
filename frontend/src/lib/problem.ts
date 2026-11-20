// Problems: how every API of the contexts says that a request failed
// (RFC 9457, TPL-0008). A screen shows a problem to the person; it never
// parses the text of one. What kind of problem it is comes from `type`, and
// which field of a form is wrong comes from `errors`.

/** A problem as the APIs send it. Each API document defines the same shape. */
export interface Problem {
  /** A URI that identifies the kind of problem and leads to its page in docs/problems/. */
  type: string;
  /** A short summary of the kind of problem. */
  title: string;
  /** The HTTP status code. */
  status: number;
  /** What went wrong in this occurrence. */
  detail?: string;
  /** A URI that identifies this occurrence. */
  instance?: string;
  /** The parts of the request that are not valid. */
  errors?: InvalidParam[];
}

/** One part of a request that is not valid. */
interface InvalidParam {
  /** A JSON Pointer to the part of the request body, or the name of a parameter. */
  pointer: string;
  /** Why the part is not valid. */
  detail: string;
}

/** What a screen does with the outcome of a call to an API. */
export type Result<Value> = { ok: true; value: Value } | { ok: false; problem: Problem };

/**
 * The kind of a problem: the name of its page in docs/problems/, such as
 * "room-already-booked". A screen that treats one kind of problem in its own
 * way compares this, not the title or the detail.
 */
export function problemKind(problem: Problem): string {
  const page = problem.type.slice(problem.type.lastIndexOf("/") + 1);
  return page.replace(/\.md$/, "");
}

/**
 * The messages of a problem for the fields of a form, by the name of the
 * field. The pointer of an invalid part is a JSON Pointer into the request
 * body ("/startsAt") or the name of a parameter ("limit"); both give the name
 * of the field. A part inside an array or an object counts for its field.
 */
export function fieldErrors(problem: Problem): Record<string, string> {
  const messages: Record<string, string> = {};
  for (const invalid of problem.errors ?? []) {
    const field = invalid.pointer.split("/").find((part) => part !== "");
    if (field !== undefined && !(field in messages)) {
      messages[field] = invalid.detail;
    }
  }
  return messages;
}

/**
 * A problem for a failure that brought no problem with it: the API did not
 * answer, or answered with something else. The screen shows it like any other
 * problem.
 */
export function unavailable(service: string, cause: unknown): Problem {
  return {
    type: "about:blank",
    title: `${service} does not answer`,
    status: 503,
    detail: cause instanceof Error ? cause.message : "The service could not be reached.",
  };
}

/** Whether a value has the shape of a problem. */
export function isProblem(value: unknown): value is Problem {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.type === "string" &&
    typeof candidate.title === "string" &&
    typeof candidate.status === "number"
  );
}
