import { connection } from "next/server";

import { blankBooking } from "../booking";
import type { Room } from "../room";
import { BookingForm } from "./booking-form";

/**
 * The part of the page of a room that books it on the date that the page
 * shows. The page renders it behind <Suspense>: it makes the Idempotency-Key
 * of the form at request time, never at build time (booking.ts says why).
 */
export async function BookingSection({
  room,
  date,
  today,
}: {
  room: Room;
  date: string;
  today: string;
}) {
  return (
    <section aria-labelledby="booking-heading">
      <h2 id="booking-heading">Book the room</h2>
      {date < today ? (
        <p>This day is over. Choose today or a later day to book the room.</p>
      ) : (
        // A form of its own for each room and date, with its own key and messages.
        <BookingForm
          key={`${room.id} ${date}`}
          room={room}
          date={date}
          initial={blankBooking(await firstKey())}
        />
      )}
    </section>
  );
}

// The first Idempotency-Key of a form: a random UUID, made for each request.
async function firstKey(): Promise<string> {
  await connection();
  return crypto.randomUUID();
}
