"use server";

// The Server Functions of Facility. Each one is a public entry point that
// anyone can post to: it narrows every value of the form before it calls the
// API, and it returns what the form shows for an expected failure instead of
// throwing. A form imports them, so they live outside server/.

import { redirect } from "next/navigation";

import { readRegistration, registrationProblem, type RegistrationState } from "./registration";
import { registerRoom as register } from "./server/rooms";

/** Registers the room that the form asks for, and goes to the page of the new room. */
export async function registerRoom(
  _previous: RegistrationState,
  form: FormData,
): Promise<RegistrationState> {
  const read = readRegistration(form);
  if (!read.ok) {
    return read.state;
  }
  const registered = await register(read.room);
  if (!registered.ok) {
    return registrationProblem(registered.problem, read.values);
  }
  redirect(`/rooms/${registered.value.id}`);
}
