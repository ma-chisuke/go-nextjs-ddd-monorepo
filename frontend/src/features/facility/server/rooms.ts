// What the screens of Facility ask of the Facility API: the meeting rooms.
// See docs/contexts/facility/.
import "server-only";

import type { components } from "@/lib/api/facility.gen";
import { facilityApi } from "@/lib/api/server/clients";
import { outcome } from "@/lib/api/server/outcome";
import type { Result } from "@/lib/problem";

/** A meeting room, as the Facility API describes it. */
export type Room = components["schemas"]["Room"];

// The largest page of the API. The sample shows one page of rooms.
const pageSize = 100;

/** The rooms, in the order of their names. */
export async function listRooms(): Promise<Result<Room[]>> {
  const page = await outcome("Facility", () =>
    facilityApi().GET("/rooms", { params: { query: { limit: pageSize } } }),
  );
  return page.ok ? { ok: true, value: page.value.items } : page;
}
