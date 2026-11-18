// What the screens of Facility ask of the Facility API: the meeting rooms.
// See docs/contexts/facility/.
import "server-only";

import { cache } from "react";

import type { components } from "@/lib/api/facility.gen";
import { facilityApi } from "@/lib/api/server/clients";
import { outcome } from "@/lib/api/server/outcome";
import type { Result } from "@/lib/problem";

import type { NewRoom } from "../registration";

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

/**
 * One room. A page and its metadata both ask for the room, so the answer is
 * kept for the rest of the request (React cache), and the API is asked once.
 */
export const getRoom = cache(async (roomId: string): Promise<Result<Room>> =>
  outcome("Facility", () => facilityApi().GET("/rooms/{roomId}", { params: { path: { roomId } } })),
);

/** Registers a room, and gives it as the API stored it. */
export async function registerRoom(room: NewRoom): Promise<Result<Room>> {
  return outcome("Facility", () => facilityApi().POST("/rooms", { body: room }));
}
