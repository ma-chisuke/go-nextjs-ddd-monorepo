"use client";

import { useActionState, useEffect, useRef } from "react";

import { Field } from "@/lib/components/field";
import { ProblemNotice } from "@/lib/components/problem-notice";
import { useFocusOnProblem } from "@/lib/components/use-focus-on-problem";

import { cancelReservation, rescheduleReservation } from "../actions";
import { unchanged } from "../changes";
import type { LocalPeriod } from "../period";
import type { ReservationField } from "../problems";
import type { Room } from "../room";

const reschedule = "reschedule";
const rescheduleNoticeId = `${reschedule}-notice`;
const cancel = "cancel";
const cancelNoticeId = `${cancel}-notice`;

/**
 * The forms that reschedule and cancel a reservation, while it is confirmed,
 * and what came of the last submit. Both forms send the version that the page
 * showed (changes.ts). After a change, the page reads the reservation again,
 * so the forms start from the reservation as it is now.
 */
export function ReservationChanges({
  reservationId,
  version,
  status,
  shown,
  room,
  period,
}: {
  reservationId: string;
  /** The version of the reservation that the page shows, as ETag gave it. */
  version: string;
  status: "confirmed" | "cancelled";
  /** The period and the attendees as the API gave them. */
  shown: { startsAt: string; endsAt: string; attendees: number };
  /** The room, or undefined when Facility does not give it now. */
  room: Room | undefined;
  /** The period on the clocks of the room, when the room is known. */
  period: LocalPeriod | undefined;
}) {
  const [rescheduled, rescheduleAction, reschedulePending] = useActionState(
    rescheduleReservation,
    unchanged,
  );
  const [cancelled, cancelAction, cancelPending] = useActionState(cancelReservation, unchanged);
  const rescheduleForm = useRef<HTMLFormElement>(null);
  const rescheduleNotice = useRef<HTMLDivElement>(null);
  const cancelForm = useRef<HTMLFormElement>(null);
  const cancelNotice = useRef<HTMLDivElement>(null);
  const doneMessage = useRef<HTMLParagraphElement>(null);
  useFocusOnProblem(rescheduled, rescheduleForm, rescheduleNotice);
  useFocusOnProblem(cancelled, cancelForm, cancelNotice);

  // After a change, the button that made it may be gone: the focus moves to
  // what happened.
  const done = cancelled.done ?? rescheduled.done;
  useEffect(() => {
    if (rescheduled.done !== undefined) {
      doneMessage.current?.focus();
    }
  }, [rescheduled]);
  useEffect(() => {
    if (cancelled.done !== undefined) {
      doneMessage.current?.focus();
    }
  }, [cancelled]);

  const about = (field: ReservationField) =>
    rescheduled.noticeFields?.includes(field) ? rescheduleNoticeId : undefined;
  const rescheduleNoticeElement = rescheduled.notice ? (
    <ProblemNotice problem={rescheduled.notice} id={rescheduleNoticeId} ref={rescheduleNotice} />
  ) : null;
  const cancelNoticeElement = cancelled.notice ? (
    <ProblemNotice problem={cancelled.notice} id={cancelNoticeId} ref={cancelNotice} />
  ) : null;

  return (
    <>
      <p role="status" className="done" ref={doneMessage} tabIndex={-1}>
        {done ?? ""}
      </p>
      {status === "cancelled" ? (
        // The forms are gone, and a notice of the last submit stays.
        <>
          {rescheduleNoticeElement}
          {cancelNoticeElement}
        </>
      ) : (
        <>
          <section aria-labelledby={`${reschedule}-heading`}>
            <h2 id={`${reschedule}-heading`}>Reschedule</h2>
            {room === undefined || period === undefined ? (
              <p>The room cannot be read now, so the reservation cannot be rescheduled here.</p>
            ) : (
              <form
                // A new version starts the form again from the reservation as it is now.
                key={version}
                action={rescheduleAction}
                ref={rescheduleForm}
                className="form"
                noValidate
              >
                <input type="hidden" name="reservationId" value={reservationId} />
                <input type="hidden" name="version" value={version} />
                <input type="hidden" name="timeZone" value={room.timeZone} />
                <input type="hidden" name="shownStartsAt" value={shown.startsAt} />
                <input type="hidden" name="shownEndsAt" value={shown.endsAt} />
                <input type="hidden" name="shownAttendees" value={shown.attendees} />
                {rescheduleNoticeElement}
                <fieldset aria-describedby={`${reschedule}-period-hint`}>
                  <legend>Period</legend>
                  <p id={`${reschedule}-period-hint`} className="hint">
                    In {room.timeZone}. The room is open from {room.openFrom} to {room.openUntil}. A
                    reservation starts and ends on a quarter hour, and lasts at most 4 hours.
                  </p>
                  <Field
                    form={reschedule}
                    name="date"
                    label="Date"
                    error={rescheduled.fieldErrors.date}
                  >
                    {(control) => (
                      <input
                        {...control}
                        type="date"
                        required
                        defaultValue={rescheduled.values?.date ?? period.date}
                      />
                    )}
                  </Field>
                  <div className="field-row">
                    <Field
                      form={reschedule}
                      name="start"
                      label="Start"
                      error={rescheduled.fieldErrors.start}
                      noticeId={about("start")}
                    >
                      {(control) => (
                        <input
                          {...control}
                          type="time"
                          required
                          defaultValue={rescheduled.values?.start ?? period.start}
                        />
                      )}
                    </Field>
                    <Field
                      form={reschedule}
                      name="end"
                      label="End"
                      error={rescheduled.fieldErrors.end}
                      noticeId={about("end")}
                    >
                      {(control) => (
                        <input
                          {...control}
                          type="time"
                          required
                          defaultValue={rescheduled.values?.end ?? period.end}
                        />
                      )}
                    </Field>
                  </div>
                </fieldset>
                <Field
                  form={reschedule}
                  name="attendees"
                  label="Attendees"
                  hint={`The number of people who attend, the organizer included. The room holds ${String(room.capacity)}.`}
                  error={rescheduled.fieldErrors.attendees}
                >
                  {(control) => (
                    <input
                      {...control}
                      type="number"
                      required
                      min={1}
                      step={1}
                      inputMode="numeric"
                      defaultValue={rescheduled.values?.attendees ?? String(shown.attendees)}
                    />
                  )}
                </Field>
                <div className="actions">
                  <button type="submit" disabled={reschedulePending || cancelPending}>
                    Reschedule
                  </button>
                  <p role="status" className="pending">
                    {reschedulePending ? "Rescheduling…" : ""}
                  </p>
                </div>
              </form>
            )}
          </section>
          <section aria-labelledby={`${cancel}-heading`}>
            <h2 id={`${cancel}-heading`}>Cancel</h2>
            <form action={cancelAction} ref={cancelForm} className="form">
              <input type="hidden" name="reservationId" value={reservationId} />
              <input type="hidden" name="version" value={version} />
              {cancelNoticeElement}
              <p className="hint">
                A reservation can be cancelled until 30 minutes before it starts. The room is then
                free for others.
              </p>
              <div className="actions">
                <button
                  type="submit"
                  className="danger"
                  disabled={reschedulePending || cancelPending}
                >
                  Cancel the reservation
                </button>
                <p role="status" className="pending">
                  {cancelPending ? "Cancelling…" : ""}
                </p>
              </div>
            </form>
          </section>
        </>
      )}
    </>
  );
}
