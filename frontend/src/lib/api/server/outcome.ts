import "server-only";

import { connection } from "next/server";

import { isProblem, unavailable, type Result } from "@/lib/problem";

/**
 * Turns one call to an API into a Result: the value of a successful answer,
 * or the problem of a failed one. An API that does not answer, or answers
 * with something that is not a problem, becomes a problem too, so a screen
 * has one way to show every failure.
 *
 * The call waits for a request. The data of the contexts is live, and the
 * build has no API to ask: without this, a call that fails before it reaches
 * the network would be rendered into the static page at build time.
 */
export async function outcome<Value>(
  service: string,
  call: () => Promise<{ data?: Value; error?: unknown }>,
): Promise<Result<Value>> {
  await connection();
  try {
    const { data, error } = await call();
    if (data !== undefined) {
      return { ok: true, value: data };
    }
    if (isProblem(error)) {
      return { ok: false, problem: error };
    }
    return {
      ok: false,
      problem: unavailable(service, new Error("The answer was not a problem (RFC 9457).")),
    };
  } catch (cause) {
    return { ok: false, problem: unavailable(service, cause) };
  }
}
