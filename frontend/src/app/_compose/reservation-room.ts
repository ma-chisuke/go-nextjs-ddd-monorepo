// The pages compose the bounded contexts; the features never import each
// other (TPL-0003). This is where a room of Facility becomes what Reservation
// needs to know of it.

import type { Room as FacilityRoom } from "@/features/facility/server/rooms";
import type { Room } from "@/features/reservation/room";
import { isTimeZone } from "@/lib/zoned-time";

/**
 * A room of Facility in the words of Reservation, or undefined when this
 * server cannot read the time zone of the room, so that no page converts a
 * time in a zone that it does not know.
 */
export function reservationRoom(room: FacilityRoom): Room | undefined {
  if (!isTimeZone(room.timeZone)) {
    return undefined;
  }
  return {
    id: room.id,
    name: room.name,
    capacity: room.capacity,
    openFrom: room.openFrom,
    openUntil: room.openUntil,
    timeZone: room.timeZone,
  };
}
