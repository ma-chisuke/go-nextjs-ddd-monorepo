import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { connection } from "next/server";
import { Suspense } from "react";

import { RoomFacts } from "@/features/facility/components/room-facts";
import { getRoom } from "@/features/facility/server/rooms";
import { BookingSection } from "@/features/reservation/components/booking-section";
import { DayReservations } from "@/features/reservation/components/day-reservations";
import { ProblemNotice } from "@/lib/components/problem-notice";
import { isUuid } from "@/lib/input";
import { problemKind } from "@/lib/problem";
import { dateIn, isDate } from "@/lib/zoned-time";

import { reservationRoom } from "../../_compose/reservation-room";

type Props = PageProps<"/rooms/[roomId]">;

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { roomId } = await params;
  const room = isUuid(roomId) ? await getRoom(roomId) : undefined;
  return { title: room?.ok ? room.value.name : "Room" };
}

/**
 * The page of a room: what the room is (Facility), and its reservations on
 * one date with a form to book it (Reservation). The date comes from the
 * search parameter `date`, and is today in the time zone of the room without
 * it.
 */
export default function RoomPage({ params, searchParams }: Props) {
  return (
    <>
      <p>
        <Link href="/">All rooms</Link>
      </p>
      <Suspense fallback={<p>Loading the room…</p>}>
        <RoomScreen params={params} searchParams={searchParams} />
      </Suspense>
    </>
  );
}

async function RoomScreen({ params, searchParams }: Pick<Props, "params" | "searchParams">) {
  const { roomId } = await params;
  if (!isUuid(roomId)) {
    notFound();
  }
  const found = await getRoom(roomId);
  if (!found.ok) {
    if (problemKind(found.problem) === "room-not-found") {
      notFound();
    }
    return (
      <>
        <h1>Room</h1>
        <ProblemNotice problem={found.problem} />
      </>
    );
  }
  const room = reservationRoom(found.value);
  const heading = (
    <>
      <h1>{found.value.name}</h1>
      <RoomFacts room={found.value} />
    </>
  );
  if (room === undefined) {
    return (
      <>
        {heading}
        <ProblemNotice
          problem={{
            title: "The time zone of this room is not known here.",
            detail: `This server cannot read times in ${found.value.timeZone}, so it shows no reservations.`,
          }}
        />
      </>
    );
  }
  const { date: asked } = await searchParams;
  // Today depends on the moment of the request, never on the build.
  await connection();
  const today = dateIn(room.timeZone, new Date());
  const date = typeof asked === "string" && isDate(asked) ? asked : today;
  return (
    <>
      {heading}
      <DayReservations room={room} date={date} today={today} />
      <BookingSection room={room} date={date} today={today} />
    </>
  );
}
