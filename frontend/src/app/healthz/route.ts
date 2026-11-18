// The health check of the frontend, for Docker Compose and ECS. Like the
// health check of a context (platform/httpserver), it says that the process
// serves requests, and it calls no API: a context that is down must not make
// the platform restart the frontend. Next.js renders the answer at build
// time, where a response needs a body, so it is 200 and not 204.
export function GET(): Response {
  return new Response("ok\n", { headers: { "content-type": "text/plain; charset=utf-8" } });
}
