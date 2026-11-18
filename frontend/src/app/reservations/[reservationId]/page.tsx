import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Suspense } from "react";

import { getRoom } from "@/features/facility/server/rooms";
import { ReservationChanges } from "@/features/reservation/components/reservation-changes";
import { ReservationDetails } from "@/features/reservation/components/reservation-details";
import { localPeriod } from "@/features/reservation/period";
import { getReservation } from "@/features/reservation/server/reservations";
import { ProblemNotice } from "@/lib/components/problem-notice";
import { isUuid } from "@/lib/input";
import { problemKind } from "@/lib/problem";

import { reservationRoom } from "../../_compose/reservation-room";

type Props = PageProps<"/reservations/[reservationId]">;

export const metadata: Metadata = { title: "Reservation" };

/**
 * The page of a reservation (Reservation), with the name, the time zone, and
 * the opening hours of its room (Facility), and the forms that reschedule and
 * cancel it. The search parameter `booked` says that the person booked it just
 * now.
 */
export default function ReservationPage({ params, searchParams }: Props) {
  return (
    <>
      <p>
        <Link href="/">All rooms</Link>
      </p>
      <Suspense fallback={<p>Loading the reservation…</p>}>
        <ReservationScreen params={params} searchParams={searchParams} />
      </Suspense>
    </>
  );
}

async function ReservationScreen({ params, searchParams }: Pick<Props, "params" | "searchParams">) {
  const { reservationId } = await params;
  if (!isUuid(reservationId)) {
    notFound();
  }
  const found = await getReservation(reservationId);
  if (!found.ok) {
    if (problemKind(found.problem) === "reservation-not-found") {
      notFound();
    }
    return (
      <>
        <h1>Reservation</h1>
        <ProblemNotice problem={found.problem} />
      </>
    );
  }
  const { reservation, version } = found.value;
  const roomFound = await getRoom(reservation.roomId);
  const room = roomFound.ok ? reservationRoom(roomFound.value) : undefined;
  const { booked } = await searchParams;
  return (
    <>
      <h1>Reservation</h1>
      {booked === "1" && reservation.status === "confirmed" ? (
        <p className="done">The room is booked.</p>
      ) : null}
      {roomFound.ok ? null : <ProblemNotice problem={roomFound.problem} />}
      <ReservationDetails reservation={reservation} room={room} />
      <ReservationChanges
        reservationId={reservation.id}
        version={version}
        status={reservation.status}
        shown={{
          startsAt: reservation.startsAt,
          endsAt: reservation.endsAt,
          attendees: reservation.attendees,
        }}
        room={room}
        period={
          room === undefined
            ? undefined
            : localPeriod(reservation.startsAt, reservation.endsAt, room.timeZone)
        }
      />
    </>
  );
}
