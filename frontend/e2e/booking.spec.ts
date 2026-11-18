// The scenarios of the sample in a browser: from a screen, through the APIs
// of Facility and Reservation, to the mail that Notification sends. They are
// the first place where the screens run with JavaScript, with the Server
// Functions, and with the streamed parts of a page.
import { expect, test } from "@playwright/test";

import { bookTomorrow, expectMail, field, organizer, registerRoom, unique } from "./support";

test("a person registers a room, books it, reschedules, and cancels, and gets the mails", async ({
  page,
  request,
}) => {
  const room = unique("E2E room");
  const email = organizer();
  await registerRoom(page, room);

  // Book.
  await bookTomorrow(page, { start: "10:00", end: "11:00", attendees: 4, organizer: email });
  await expect(page).toHaveURL(/\/reservations\/[0-9a-f-]{36}\?booked=1$/);
  await expect(page.getByText("The room is booked.")).toBeVisible();
  const main = page.getByRole("main");
  await expect(main.getByText("Confirmed", { exact: true })).toBeVisible();
  await expect(main.getByText(/10:00 to 11:00 \(Asia\/Tokyo\)/)).toBeVisible();
  await expect(main.getByRole("link", { name: room })).toBeVisible();

  // Reschedule: the page sends the version that it showed (If-Match), and
  // then shows the reservation as the API has it now.
  await field(page, "Start").fill("11:00");
  await field(page, "End").fill("12:00");
  await page.getByRole("button", { name: "Reschedule" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "The reservation is rescheduled." }),
  ).toBeVisible();
  await expect(main.getByText(/11:00 to 12:00 \(Asia\/Tokyo\)/)).toBeVisible();

  // Cancel, with the version after the rescheduling.
  await page.getByRole("button", { name: "Cancel the reservation" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "The reservation is cancelled." }),
  ).toBeVisible();
  await expect(main.getByText("Cancelled", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Reschedule" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Cancel the reservation" })).toHaveCount(0);

  // The room shows the reservation of that day as cancelled.
  await main.getByRole("link", { name: room }).click();
  await expect(page.getByRole("heading", { level: 1, name: room })).toBeVisible();

  // Each change wrote an event to the outbox of Reservation; Notification
  // turned each into one mail.
  await expectMail(request, email, [
    "Reservation confirmed",
    "Reservation confirmed",
    "Reservation cancelled",
  ]);
});

test("a second booking of the same period is refused in the words of the screen", async ({
  page,
}) => {
  const room = unique("E2E room");
  await registerRoom(page, room);
  const roomUrl = page.url();

  await bookTomorrow(page, { start: "13:00", end: "14:00", attendees: 2, organizer: organizer() });
  await expect(page.getByText("The room is booked.")).toBeVisible();

  // Another person asks for a period that overlaps the first one.
  await page.goto(roomUrl);
  await bookTomorrow(page, { start: "13:30", end: "14:30", attendees: 2, organizer: organizer() });
  await expect(
    page
      .getByRole("main")
      .getByRole("alert")
      .filter({ hasText: "The room is already booked for part of this period." }),
  ).toBeVisible();
  // Nothing was booked: the person is still on the page of the room, with
  // the first reservation in the list of the day.
  await expect(page).toHaveURL(/\/rooms\//);
  await expect(page.getByRole("link", { name: "13:00 to 14:00" })).toBeVisible();
  await expect(page.getByRole("link", { name: "13:30 to 14:30" })).toHaveCount(0);
});

test("a change from a page that is out of date is refused, and the page shows the reservation as it is now", async ({
  page,
  context,
}) => {
  const room = unique("E2E room");
  await registerRoom(page, room);
  await bookTomorrow(page, { start: "15:00", end: "16:00", attendees: 3, organizer: organizer() });
  await expect(page.getByText("The room is booked.")).toBeVisible();

  // A second person opens the same reservation, and reschedules it.
  const other = await context.newPage();
  await other.goto(page.url());
  await field(other, "Start").fill("16:00");
  await field(other, "End").fill("17:00");
  await other.getByRole("button", { name: "Reschedule" }).click();
  await expect(
    other.getByRole("status").filter({ hasText: "The reservation is rescheduled." }),
  ).toBeVisible();

  // The first person still sees the old period, and cancels. The page sends
  // the version that it showed, the API answers 412, and nothing is
  // cancelled.
  await page.getByRole("button", { name: "Cancel the reservation" }).click();
  const main = page.getByRole("main");
  await expect(
    main
      .getByRole("alert")
      .filter({ hasText: "Somebody changed this reservation after you opened this page." }),
  ).toBeVisible();
  await expect(main.getByText(/16:00 to 17:00 \(Asia\/Tokyo\)/)).toBeVisible();
  await expect(main.getByText("Confirmed", { exact: true })).toBeVisible();
});

test("a form that is not filled in says so at its fields", async ({ page }) => {
  await page.goto("/rooms/new");
  await field(page, "Capacity").fill("0");
  await page.getByRole("button", { name: "Register the room" }).click();

  const name = field(page, "Name");
  await expect(name).toHaveAttribute("aria-invalid", "true");
  await expect(name).toHaveAccessibleDescription(/Enter the name of the room\./);
  await expect(field(page, "Capacity")).toHaveAccessibleDescription(
    /Enter a whole number from 1 to 1000\./,
  );
  // The focus is on the first field that is not valid, so that a person who
  // uses the keyboard or a screen reader lands on what to fix.
  await expect(name).toBeFocused();
  await expect(page).toHaveURL(/\/rooms\/new$/);
});
