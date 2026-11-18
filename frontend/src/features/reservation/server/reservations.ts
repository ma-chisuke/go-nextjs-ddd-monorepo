// What the screens of Reservation ask of the Reservation API: reservations,
// with their versions, and the changes to them. See docs/contexts/reservation/.
import "server-only";

import { cache } from "react";

import type { components } from "@/lib/api/reservation.gen";
import { reservationApi } from "@/lib/api/server/clients";
import { outcome } from "@/lib/api/server/outcome";
import { unavailable, type Result } from "@/lib/problem";

import type { NewReservation } from "../booking";
import { isVersion, type ReservationChanges, type Target } from "../changes";

/** A reservation, as the Reservation API describes it. */
export type Reservation = components["schemas"]["Reservation"];

/** A reservation, and its version as ETag gave it. A change names the version in If-Match. */
export interface Versioned {
  reservation: Reservation;
  version: string;
}

const service = "Reservation";

// The largest page of the API.
const pageSize = 100;

// The most pages that one list reads: a day of one room holds at most 96
// confirmed reservations of a quarter hour, and few cancelled ones.
const mostPages = 10;

/**
 * One reservation, with its version. A page and its metadata both ask for
 * it, so the answer is kept for the rest of the request (React cache).
 */
export const getReservation = cache(async (reservationId: string): Promise<Result<Versioned>> =>
  outcome(service, async () => {
    const { data, error, response } = await reservationApi().GET("/reservations/{reservationId}", {
      params: { path: { reservationId } },
    });
    if (data === undefined) {
      return { error };
    }
    const version = response.headers.get("ETag") ?? undefined;
    if (!isVersion(version)) {
      // Without its version, the page could not change the reservation.
      return { error: unavailable(service, new Error("The answer carried no ETag.")) };
    }
    return { data: { reservation: data, version } };
  }),
);

/**
 * The reservations of a room, confirmed and cancelled, whose period overlaps
 * the window from `from` to `to`, in the order of their start.
 */
export async function listReservations(
  roomId: string,
  window: { from: Date; to: Date },
): Promise<Result<Reservation[]>> {
  const reservations: Reservation[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < mostPages; page++) {
    const answer = await outcome(service, () =>
      reservationApi().GET("/reservations", {
        params: {
          query: {
            roomId,
            from: window.from.toISOString(),
            to: window.to.toISOString(),
            limit: pageSize,
            cursor,
          },
        },
      }),
    );
    if (!answer.ok) {
      return answer;
    }
    reservations.push(...answer.value.items);
    if (answer.value.nextCursor === null) {
      break;
    }
    cursor = answer.value.nextCursor;
  }
  return { ok: true, value: reservations };
}

/**
 * Books a room. A request with an Idempotency-Key that booked before, with
 * the same booking, gets the reservation of that booking.
 */
export async function bookRoom(
  booking: NewReservation,
  idempotencyKey: string,
): Promise<Result<Reservation>> {
  return outcome(service, () =>
    reservationApi().POST("/reservations", {
      params: { header: { "Idempotency-Key": idempotencyKey } },
      body: booking,
    }),
  );
}

/** Changes the period or the attendees of the version of a reservation that the page showed. */
export async function rescheduleReservation(
  target: Target,
  changes: ReservationChanges,
): Promise<Result<Reservation>> {
  return outcome(service, () =>
    reservationApi().PATCH("/reservations/{reservationId}", {
      params: {
        path: { reservationId: target.reservationId },
        header: { "If-Match": target.version },
      },
      body: changes,
    }),
  );
}

/** Cancels the version of a reservation that the page showed. */
export async function cancelReservation(target: Target): Promise<Result<Reservation>> {
  return outcome(service, () =>
    reservationApi().POST("/reservations/{reservationId}/cancellation", {
      params: {
        path: { reservationId: target.reservationId },
        header: { "If-Match": target.version },
      },
    }),
  );
}
