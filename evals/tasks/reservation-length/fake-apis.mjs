// The Facility API and the Reservation API as the page of a room asks them,
// with fixed answers, for the grader of the task reservation-length (evals/).
// One server answers both: their paths do not collide.
//
// Usage: node fake-apis.mjs <port>
//
// The room is in Tokyo. On 20 November 2026 it has three reservations: from
// 10:00 to 11:30 (90 minutes), from 13:00 to 13:45 (45 minutes), and from
// 15:00 to 17:00 (120 minutes), local time. The answers follow the schemas
// of contracts/http/facility.openapi.yaml and reservation.openapi.yaml.
import { createServer } from "node:http";

const roomId = "0193a6f0-1a2b-7c3d-8e4f-00000000e001";

const room = {
  id: roomId,
  name: "Eval room",
  capacity: 8,
  openFrom: "08:00",
  openUntil: "20:00",
  timeZone: "Asia/Tokyo",
  createdAt: "2026-11-01T00:00:00Z",
};

const reservation = (n, startsAt, endsAt) => ({
  id: `0193a6f0-7b5c-7e2a-9c3d-00000000e00${String(n)}`,
  roomId,
  startsAt,
  endsAt,
  attendees: 4,
  organizerEmail: "organizer@example.com",
  status: "confirmed",
});

const reservations = [
  reservation(1, "2026-11-20T01:00:00Z", "2026-11-20T02:30:00Z"),
  reservation(2, "2026-11-20T04:00:00Z", "2026-11-20T04:45:00Z"),
  reservation(3, "2026-11-20T06:00:00Z", "2026-11-20T08:00:00Z"),
];

const json = (response, status, body, type = "application/json") => {
  response.writeHead(status, { "content-type": type });
  response.end(JSON.stringify(body));
};

const problem = (response, status, slug, title) =>
  json(
    response,
    status,
    { type: `https://example.com/problems/${slug}.md`, title, status },
    "application/problem+json",
  );

const server = createServer((request, response) => {
  const url = new URL(request.url ?? "/", "http://fake");
  process.stderr.write(`fake-apis: ${request.method ?? "?"} ${url.pathname}${url.search}\n`);
  if (request.method !== "GET") {
    problem(response, 405, "method-not-allowed", "The method is not allowed");
  } else if (url.pathname === `/rooms/${roomId}`) {
    json(response, 200, room);
  } else if (url.pathname.startsWith("/rooms/")) {
    problem(response, 404, "room-not-found", "The room does not exist");
  } else if (url.pathname === "/reservations") {
    json(response, 200, { items: reservations, nextCursor: null });
  } else {
    problem(response, 404, "not-found", "The resource does not exist");
  }
});

server.listen(Number(process.argv[2]), "127.0.0.1");
