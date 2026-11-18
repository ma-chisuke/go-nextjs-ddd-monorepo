import type { Ref } from "react";

import type { Notice } from "@/lib/input";

/**
 * Shows a problem that an API answered with (RFC 9457), or what a form says
 * about one in its own words: a title, and what went wrong this time. Screen
 * readers announce it when it appears. A form moves the focus to it after a
 * failed submit that no field explains, so it takes the focus by script but
 * stays out of the order of the Tab key.
 */
export function ProblemNotice({
  problem,
  id,
  ref,
}: {
  problem: Notice;
  id?: string;
  ref?: Ref<HTMLDivElement>;
}) {
  return (
    <div role="alert" className="problem" id={id} ref={ref} tabIndex={-1}>
      <p className="problem-title">{problem.title}</p>
      {problem.detail ? <p>{problem.detail}</p> : null}
    </div>
  );
}
