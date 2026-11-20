// StrykerJS: mutation testing of the pure logic of the frontend. It changes
// the code of each module that has a unit test next to it, one change (a
// mutant) at a time, and runs the unit tests against each mutant.
// `make test-mutation-frontend` runs it, and tools/stryker/findings.jq turns
// its report into findings. See docs/guides/mutation-tests.md.
// https://stryker-mutator.io/docs/stryker-js/configuration/
import { globSync } from "node:fs";

// The unit tests, and the modules next to them: src/lib/input.test.ts tests
// src/lib/input.ts. A new module with a test is mutated without an edit here.
// Components and server modules have no unit test; the browser tests them
// (make e2e).
const tests = globSync("src/**/*.test.ts").sort();

const config = {
  // pnpm installs each package in a directory of its own, so Stryker cannot
  // find its test runner by a pattern, and loads it by name.
  plugins: ["@stryker-mutator/tap-runner"],
  // The test runner of Node, as make test-frontend runs it, with each test
  // file in a process of its own. Stryker reads the results as TAP.
  testRunner: "tap",
  tap: { testFiles: tests, nodeArgs: ["--test-reporter=tap"] },
  mutate: tests.map((test) => test.replace(/\.test\.ts$/, ".ts")),
  // A mutant runs only the test files that run its line.
  coverageAnalysis: "perTest",
  // A mutant that makes a test hang counts as killed, because a test noticed
  // it. Stryker stops an endless loop by counting how often the mutant runs,
  // which does not depend on the machine; anything else it stops by time. The
  // floor of that time is high, so that a busy machine, which starts the
  // process of a test file slowly, does not count a slow test as a hang and
  // hide a mutant that survives.
  timeoutMS: 30_000,
  reporters: ["json"],
  // tools/scripts/mutation.sh sets the report of a run of one mutant.
  jsonReporter: {
    fileName: process.env.STRYKER_REPORT ?? "../.cache/mutation/frontend.json",
  },
  // The copy of the sources that the mutants change, out of the sources.
  tempDirName: "../.cache/mutation/stryker",
  allowConsoleColors: false,
};

export default config;
