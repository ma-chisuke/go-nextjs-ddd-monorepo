// What the scenarios share: a room and a booking made through the screens,
// as a person makes them, and the mail that Mailpit caught.
import { randomUUID } from "node:crypto";

import { expect, type APIRequestContext, type Locator, type Page } from "@playwright/test";

/** A name or an address that no other run uses, so that scenarios never meet. */
export function unique(prefix: string): string {
  return `${prefix}-${randomUUID()}`;
}

/** The email address of an organizer of this scenario alone. */
export function organizer(): string {
  return `${unique("e2e")}@example.com`;
}

/**
 * Registers a room through the screens, open from 08:00 to 20:00 in Tokyo for
 * six people, and ends on the page of the room.
 */
export async function registerRoom(page: Page, name: string): Promise<void> {
  await page.goto("/");
  await page.getByRole("link", { name: "Register a room" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Register a room" })).toBeVisible();
  await field(page, "Name").fill(name);
  await field(page, "Capacity").fill("6");
  await field(page, "Opens at").fill("08:00");
  await field(page, "Closes at").fill("20:00");
  await field(page, "Time zone").selectOption("Asia/Tokyo");
  await page.getByRole("button", { name: "Register the room" }).click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
}

/**
 * The field with the label, on the page that a person sees. Next.js keeps the
 * page that a person came from in the document, hidden, so that going back
 * restores it as it was. A label of that page, such as "Start" of the booking
 * form, would otherwise match next to the same label of the page in front.
 */
export function field(page: Page, label: string): Locator {
  return page.getByLabel(label, { exact: true }).filter({ visible: true });
}

/** What a person types into the booking form. */
export interface Booking {
  start: string;
  end: string;
  attendees: number;
  organizer: string;
}

/**
 * On the page of a room, goes to the next day, so that the period lies in the
 * future whatever the time is, and submits the booking form.
 */
export async function bookTomorrow(page: Page, booking: Booking): Promise<void> {
  await page.getByRole("link", { name: "Next day" }).click();
  await expect(page).toHaveURL(/[?&]date=\d{4}-\d{2}-\d{2}/);
  await field(page, "Start").fill(booking.start);
  await field(page, "End").fill(booking.end);
  await field(page, "Attendees").fill(String(booking.attendees));
  await field(page, "Organizer's email").fill(booking.organizer);
  await page.getByRole("button", { name: "Book the room" }).click();
}

// Mailpit, which catches the mail of the local system. `make e2e` passes its
// address; the default is its port in compose.yaml.
const mailpit = process.env.MAILPIT_URL ?? "http://127.0.0.1:18025";

/**
 * Waits until Mailpit holds exactly these mails to the address, by what their
 * subjects say before the colon, in any order. The events travel through the
 * outbox, the event bus, and the queue, so the mail arrives a little later
 * than the screen changes.
 */
export async function expectMail(
  request: APIRequestContext,
  address: string,
  subjects: string[],
): Promise<void> {
  const search = `${mailpit}/api/v1/search?query=${encodeURIComponent(`to:"${address}"`)}`;
  await expect
    .poll(
      async () => {
        const response = await request.get(search);
        expect(response.ok(), `Mailpit at ${mailpit} answers`).toBe(true);
        const found = (await response.json()) as { messages: { Subject: string }[] };
        return found.messages.map((message) => message.Subject.split(":")[0]).sort();
      },
      { timeout: 60_000, message: `the mails to ${address}` },
    )
    .toEqual([...subjects].sort());
}
