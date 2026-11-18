import { expect, test } from "@playwright/test";

test("the health check answers without the APIs", async ({ request }) => {
  const response = await request.get("/healthz");
  expect(response.status()).toBe(200);
});

test("the first screen lists the rooms of Facility", async ({ page }) => {
  await page.goto("/");
  const main = page.getByRole("main");
  await expect(main.getByRole("heading", { level: 1, name: "Rooms" })).toBeVisible();
  // The list streams in from the Facility API after the static shell: either
  // rooms, or the sentence of an empty system, but never a problem.
  await expect(main.getByText("Loading the rooms…")).toHaveCount(0);
  await expect(main.getByRole("alert")).toHaveCount(0);
});
