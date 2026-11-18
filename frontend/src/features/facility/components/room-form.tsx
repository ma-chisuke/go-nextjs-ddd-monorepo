"use client";

import { useActionState, useRef, useSyncExternalStore } from "react";

import { Field } from "@/lib/components/field";
import { ProblemNotice } from "@/lib/components/problem-notice";
import { useFocusOnProblem } from "@/lib/components/use-focus-on-problem";

import { registerRoom } from "../actions";
import { blankRegistration, type RoomField } from "../registration";

const form = "room";
const noticeId = `${form}-notice`;

/**
 * The form that registers a room. The Server Function checks every field and
 * answers with what the form shows, so the browser does not validate on its
 * own (noValidate): every message appears in the same place, tied to its
 * field, with or without JavaScript.
 */
export function RoomForm({ timeZones }: { timeZones: string[] }) {
  const [state, action, pending] = useActionState(registerRoom, blankRegistration);
  const formRef = useRef<HTMLFormElement>(null);
  const noticeRef = useRef<HTMLDivElement>(null);
  useFocusOnProblem(state, formRef, noticeRef);
  const browserZone = useBrowserTimeZone();
  const zone = state.values?.timeZone ?? browserZone ?? "UTC";
  const zones = timeZones.includes(zone) ? timeZones : [zone, ...timeZones];
  const about = (field: RoomField) => (state.noticeFields?.includes(field) ? noticeId : undefined);

  return (
    <form action={action} ref={formRef} className="form" noValidate>
      {state.notice ? <ProblemNotice problem={state.notice} id={noticeId} ref={noticeRef} /> : null}
      <Field
        form={form}
        name="name"
        label="Name"
        hint="The name that people use for the room. No two rooms have the same name."
        error={state.fieldErrors.name}
        noticeId={about("name")}
      >
        {(control) => (
          <input
            {...control}
            type="text"
            required
            maxLength={100}
            autoComplete="off"
            defaultValue={state.values?.name ?? ""}
          />
        )}
      </Field>
      <Field
        form={form}
        name="capacity"
        label="Capacity"
        hint="The largest number of people the room holds, from 1 to 1000."
        error={state.fieldErrors.capacity}
      >
        {(control) => (
          <input
            {...control}
            type="number"
            required
            min={1}
            max={1000}
            step={1}
            inputMode="numeric"
            defaultValue={state.values?.capacity ?? ""}
          />
        )}
      </Field>
      <fieldset aria-describedby={`${form}-hours-hint`}>
        <legend>Opening hours</legend>
        <p id={`${form}-hours-hint`} className="hint">
          Every day, on a 24-hour clock, in the time zone of the room. A room closes on the day it
          opens.
        </p>
        <div className="field-row">
          <Field form={form} name="openFrom" label="Opens at" error={state.fieldErrors.openFrom}>
            {(control) => (
              <input
                {...control}
                type="time"
                required
                defaultValue={state.values?.openFrom ?? "09:00"}
              />
            )}
          </Field>
          <Field form={form} name="openUntil" label="Closes at" error={state.fieldErrors.openUntil}>
            {(control) => (
              <input
                {...control}
                type="time"
                required
                defaultValue={state.values?.openUntil ?? "18:00"}
              />
            )}
          </Field>
        </div>
      </fieldset>
      <Field
        form={form}
        name="timeZone"
        label="Time zone"
        hint="The IANA time zone in which the opening hours and the reservations of the room are read."
        error={state.fieldErrors.timeZone}
      >
        {(control) => (
          // A new key makes a new select when the zone to choose first is
          // known: the zone of the browser, known only once it runs here, or
          // the zone of the last submit.
          <select {...control} key={zone} required defaultValue={zone}>
            {zones.map((name) => (
              <option key={name} value={name}>
                {name}
              </option>
            ))}
          </select>
        )}
      </Field>
      <div className="actions">
        <button type="submit" disabled={pending}>
          Register the room
        </button>
        <p role="status" className="pending">
          {pending ? "Registering the room…" : ""}
        </p>
      </div>
    </form>
  );
}

// The time zone of the browser. The server renders the form without it, and
// the browser chooses it once it runs, without a mismatch on hydration.
const noChanges = () => () => undefined;

function useBrowserTimeZone(): string | undefined {
  return useSyncExternalStore(
    noChanges,
    () => Intl.DateTimeFormat().resolvedOptions().timeZone,
    () => undefined,
  );
}
