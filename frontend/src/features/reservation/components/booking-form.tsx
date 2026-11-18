"use client";

import { useActionState, useRef } from "react";

import { Field } from "@/lib/components/field";
import { ProblemNotice } from "@/lib/components/problem-notice";
import { useFocusOnProblem } from "@/lib/components/use-focus-on-problem";

import { bookRoom } from "../actions";
import type { BookingState } from "../booking";
import type { ReservationField } from "../problems";
import type { Room } from "../room";

const form = "booking";
const noticeId = `${form}-notice`;

/**
 * The form that books a room on one date, with times of day in the time zone
 * of the room. The hidden fields carry the room, the date, the zone, and the
 * Idempotency-Key with the booking that last went out with it (booking.ts).
 * The Server Function checks every field, so the browser does not validate on
 * its own (noValidate).
 */
export function BookingForm({
  room,
  date,
  initial,
}: {
  room: Room;
  date: string;
  initial: BookingState;
}) {
  const [state, action, pending] = useActionState(bookRoom, initial);
  const formRef = useRef<HTMLFormElement>(null);
  const noticeRef = useRef<HTMLDivElement>(null);
  useFocusOnProblem(state, formRef, noticeRef);
  const about = (field: ReservationField) =>
    state.noticeFields?.includes(field) ? noticeId : undefined;

  return (
    <form action={action} ref={formRef} className="form" noValidate>
      <input type="hidden" name="roomId" value={room.id} />
      <input type="hidden" name="date" value={date} />
      <input type="hidden" name="timeZone" value={room.timeZone} />
      <input type="hidden" name="idempotencyKey" value={state.idempotencyKey} />
      <input type="hidden" name="sentWith" value={state.sentWith ?? ""} />
      {state.notice ? <ProblemNotice problem={state.notice} id={noticeId} ref={noticeRef} /> : null}
      <fieldset aria-describedby={`${form}-period-hint`}>
        <legend>Period</legend>
        <p id={`${form}-period-hint`} className="hint">
          In {room.timeZone}. The room is open from {room.openFrom} to {room.openUntil}. A
          reservation starts and ends on a quarter hour, and lasts at most 4 hours.
        </p>
        <div className="field-row">
          <Field
            form={form}
            name="start"
            label="Start"
            error={state.fieldErrors.start}
            noticeId={about("start")}
          >
            {(control) => (
              <input {...control} type="time" required defaultValue={state.values?.start ?? ""} />
            )}
          </Field>
          <Field
            form={form}
            name="end"
            label="End"
            error={state.fieldErrors.end}
            noticeId={about("end")}
          >
            {(control) => (
              <input {...control} type="time" required defaultValue={state.values?.end ?? ""} />
            )}
          </Field>
        </div>
      </fieldset>
      <Field
        form={form}
        name="attendees"
        label="Attendees"
        hint={`The number of people who attend, the organizer included. The room holds ${String(room.capacity)}.`}
        error={state.fieldErrors.attendees}
      >
        {(control) => (
          <input
            {...control}
            type="number"
            required
            min={1}
            step={1}
            inputMode="numeric"
            defaultValue={state.values?.attendees ?? ""}
          />
        )}
      </Field>
      <Field
        form={form}
        name="organizerEmail"
        label="Organizer's email"
        hint="The confirmation of the booking goes to this address."
        error={state.fieldErrors.organizerEmail}
      >
        {(control) => (
          <input
            {...control}
            type="email"
            required
            autoComplete="email"
            defaultValue={state.values?.organizerEmail ?? ""}
          />
        )}
      </Field>
      <div className="actions">
        <button type="submit" disabled={pending}>
          Book the room
        </button>
        <p role="status" className="pending">
          {pending ? "Booking the room…" : ""}
        </p>
      </div>
    </form>
  );
}
