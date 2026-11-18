// The health check of the container image of the frontend
// (frontend/Dockerfile): it asks the server that runs in this container for
// GET /healthz, and exits with 0 when the server answers with a 2xx status
// and with 1 when it does not. The image has no shell or curl, so Node runs
// this file, in Docker Compose and in ECS.
const port = process.env.PORT ?? "3000";
const url = `http://127.0.0.1:${port}/healthz`;

try {
  const response = await fetch(url, { signal: AbortSignal.timeout(2000) });
  if (!response.ok) {
    console.error(`the health check failed: GET ${url} answered ${String(response.status)}`);
    process.exit(1);
  }
} catch (cause) {
  console.error(
    `the health check failed: GET ${url}: ${cause instanceof Error ? cause.message : String(cause)}`,
  );
  process.exit(1);
}
