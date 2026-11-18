// The clients of the APIs of the bounded contexts, typed by the API documents
// (TPL-0006). Only the server of Next.js calls the APIs: they live on the
// private network, and the browser talks to Next.js alone (TPL-0014). So this
// module is server-only, and a client component that imports it fails the
// build.
import "server-only";

import createClient, { type Client } from "openapi-fetch";

import type { paths as FacilityPaths } from "../facility.gen";
import type { paths as ReservationPaths } from "../reservation.gen";

// How long one call to an API may take. A page waits for its data, so an API
// that hangs must not hang the page with it.
const timeoutMs = 5000;

// The base URL of an API comes from the environment at run time, so that one
// image runs in every environment.
function baseUrl(variable: string): string {
  const value = process.env[variable];
  if (!value) {
    throw new Error(
      `${variable} is not set: it holds the base URL of an API, such as http://facility:8080`,
    );
  }
  return value;
}

function client<Paths extends object>(variable: string): Client<Paths> {
  return createClient<Paths>({
    baseUrl: baseUrl(variable),
    fetch: (request) => fetch(request, { signal: AbortSignal.timeout(timeoutMs) }),
  });
}

/** The client of the Facility API. */
export function facilityApi(): Client<FacilityPaths> {
  return client<FacilityPaths>("FACILITY_API_URL");
}

/** The client of the Reservation API. */
export function reservationApi(): Client<ReservationPaths> {
  return client<ReservationPaths>("RESERVATION_API_URL");
}
