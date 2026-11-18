import Link from "next/link";
import { Suspense } from "react";

import { ProblemNotice } from "@/lib/components/problem-notice";
import { addDays, dayWindow, formatDate } from "@/lib/zoned-time";

import { formatTimes, localPeriod } from "../period";
import type { Room } from "../room";
import { listReservations } from "../server/reservations";

/**
 * The reservations of a room on one date in the time zone of the room, with
 * links to the dates before and after. The list streams in behind its own
 * <Suspense>, so the heading and the links show at once.
 */
export function DayReservations({
  room,
  date,
  today,
}: {
  room: Room;
  date: string;
  today: string;
}) {
  return (
    <section aria-labelledby="day-heading">
      <h2 id="day-heading">Reservations on {formatDate(date)}</h2>
      <nav aria-label="Dates" className="day-nav">
        <Link href={`/rooms/${room.id}?date=${addDays(date, -1)}`}>Previous day</Link>
        {date === today ? null : <Link href={`/rooms/${room.id}`}>Today</Link>}
        <Link href={`/rooms/${room.id}?date=${addDays(date, 1)}`}>Next day</Link>
      </nav>
      <Suspense fallback={<p>Loading the reservations…</p>}>
        <ReservationList room={room} date={date} />
      </Suspense>
    </section>
  );
}

async function ReservationList({ room, date }: { room: Room; date: string }) {
  const reservations = await listReservations(room.id, dayWindow(date, room.timeZone));
  if (!reservations.ok) {
    return <ProblemNotice problem={reservations.problem} />;
  }
  if (reservations.value.length === 0) {
    return <p>No reservations on this day.</p>;
  }
  return (
    <ul className="reservations" aria-labelledby="day-heading">
      {reservations.value.map((reservation) => {
        const times = formatTimes(
          localPeriod(reservation.startsAt, reservation.endsAt, room.timeZone),
          date,
        );
        return (
          <li key={reservation.id} className={`reservation ${reservation.status}`}>
            <Link href={`/reservations/${reservation.id}`} className="reservation-times">
              {times}
            </Link>
            <span className="reservation-status">
              {reservation.status === "confirmed" ? "Confirmed" : "Cancelled"}
            </span>
            <span className="reservation-facts">
              {reservation.attendees === 1
                ? "1 attendee"
                : `${String(reservation.attendees)} attendees`}
              , organized by {reservation.organizerEmail}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
