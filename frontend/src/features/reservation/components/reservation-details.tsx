import Link from "next/link";

import { formatPeriod, localPeriod } from "../period";
import type { Room } from "../room";
import type { Reservation } from "../server/reservations";

/**
 * What a reservation is: its room, its period in the time zone of the room,
 * its attendees, its organizer, and its status. Without the room, which
 * Facility may fail to give, the period shows in UTC.
 */
export function ReservationDetails({
  reservation,
  room,
}: {
  reservation: Reservation;
  room: Room | undefined;
}) {
  const timeZone = room?.timeZone ?? "UTC";
  const period = localPeriod(reservation.startsAt, reservation.endsAt, timeZone);
  return (
    <dl className="details">
      <dt>Room</dt>
      <dd>
        {room ? (
          <Link href={`/rooms/${room.id}`}>{room.name}</Link>
        ) : (
          "The room cannot be read now."
        )}
      </dd>
      <dt>When</dt>
      <dd>
        {formatPeriod(period)} ({timeZone})
      </dd>
      <dt>Attendees</dt>
      <dd>{reservation.attendees}</dd>
      <dt>Organizer</dt>
      <dd>{reservation.organizerEmail}</dd>
      <dt>Status</dt>
      <dd>
        <span className={`status-badge ${reservation.status}`}>
          {reservation.status === "confirmed" ? "Confirmed" : "Cancelled"}
        </span>
      </dd>
    </dl>
  );
}
