// Playwright runs the end-to-end tests in frontend/e2e against the local
// system in Docker Compose. `make e2e` starts the system and passes its
// addresses; the defaults are the ports of compose.yaml, for a run against a
// system that `make up` started. https://playwright.dev/docs/test-configuration
import { defineConfig, devices } from "@playwright/test";

const cache = "../.cache/playwright";

export default defineConfig({
  testDir: "e2e",
  // What a failed test leaves behind (traces, screenshots) stays out of the
  // sources.
  outputDir: `${cache}/results`,
  reporter: [["list"], ["html", { outputFolder: `${cache}/report`, open: "never" }]],
  // A test that only passes on a second try has a bug to find.
  retries: 0,
  // `test.only` left in a file must not hide the other tests in CI.
  forbidOnly: process.env.CI !== undefined,
  // The scenarios share one system, and each one makes its own room, so they
  // run side by side.
  fullyParallel: true,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: process.env.WEB_URL ?? "http://127.0.0.1:53000",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  // One browser. The scenarios test the path from a screen to the mail, not
  // the differences between browsers.
  //
  // The channel "chromium" is the whole browser in its headless mode, not the
  // smaller headless shell that Playwright takes by default. The shell fired
  // no animation frame here, and React waits for one before it shows what
  // streamed in behind a <Suspense> boundary, so every page stayed at its
  // fallback. tools/scripts/stack.sh installs this browser alone.
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], channel: "chromium" } }],
});
