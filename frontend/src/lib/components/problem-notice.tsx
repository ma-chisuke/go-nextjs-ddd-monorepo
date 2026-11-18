import type { Problem } from "@/lib/problem";

/**
 * Shows a problem that an API answered with (RFC 9457): its title, and what
 * went wrong this time. Screen readers announce it when it appears.
 */
export function ProblemNotice({ problem }: { problem: Problem }) {
  return (
    <div role="alert" className="problem">
      <p className="problem-title">{problem.title}</p>
      {problem.detail ? <p>{problem.detail}</p> : null}
    </div>
  );
}
