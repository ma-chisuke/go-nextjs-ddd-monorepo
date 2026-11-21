# The records of the evaluation

Each file `<time>-<task>-<arm>.json` in this directory is one run of `make eval`: the task, the arm, the commit, the model, the time, what the agent used, and each criterion with its result. `make eval-report` prints them as a table.

## What the runs so far showed

Twelve runs were made on November 20, 2026, with the model `sonnet`: each of the six tasks once in each arm, in two rounds.

Twelve runs prove nothing: an agent does not repeat itself, and one run in each arm cannot tell a difference from chance. They are a first look. They show that the evaluation works from end to end, and where to look next.

### The first round: two small tasks

The first round used Claude Code 2.1.327.

| Task | Arm | Result | Criteria passed | Turns | Seconds | Refused commands |
|---|---|---|---|---|---|---|
| `read-the-logs` | `harness` | Passed | 5 of 5 | 8 | 105 | 1 |
| `read-the-logs` | `bare` | Passed | 5 of 5 | 10 | 25 | 2 |
| `booking-horizon` | `harness` | Passed | 7 of 7 | 50 | 240 | 3 |
| `booking-horizon` | `bare` | Failed | 6 of 7 | 52 | 130 | 10 |

What the round showed:

- **Without the harness, the agent did the work and did not finish it.** In `booking-horizon`, both arms wrote the rule in the domain with its test, the type of problem, its page in both languages, the README of the context, and the words on the screen, and both described the rule in the API document, which the task did not ask for. Without the harness, the agent stopped there: the code generated from the API document was stale in two files, a Go file was not formatted, and two sentences of a translation broke the rules of the Japanese prose. `make check` reports all five; the agent never ran it. With the harness, the agent ran `make gen` and `make check` before it stopped. This is the case that TPL-0020 argues from: the checks hold the rules, and the harness has little more to do than to make the agent run them.
- **The translation followed in both arms.** In `read-the-logs`, the agent without the harness also wrote the Japanese section, and brought the marker of the translation up to date by computing the hash itself, since nothing told it of `make i18n-stamp`. The guides of the repository stay in the bare arm, and they were enough here.
- **Nothing was weakened.** No run touched an allowance file, a hook, or a check, or added an ignore comment.
- **No criterion failed with the harness.** So this round gave no reason to add a sentence to `AGENTS.md`.

### The second round: four larger tasks

The second round used Claude Code 2.1.328. Its tasks are the kinds of change that the guides describe: an endpoint (`rename-room`), a schema change (`room-floor`), an event (`time-zone-in-mail`), and a screen (`reservation-length`).

| Task | Arm | Result | Criteria passed | Seconds | Output tokens | Refused commands |
|---|---|---|---|---|---|---|
| `rename-room` | `harness` | Passed | 7 of 7 | 1998 | 19422 | 0 |
| `rename-room` | `bare` | Passed | 7 of 7 | 547 | 14005 | 5 |
| `room-floor` | `harness` | Failed | 5 of 6 | 2480 | 15077 | 0 |
| `room-floor` | `bare` | Failed | 5 of 6 | 1137 | 19524 | 0 |
| `time-zone-in-mail` | `harness` | Failed | 5 of 6 | 1506 | 17466 | 0 |
| `time-zone-in-mail` | `bare` | Failed | 4 of 6 | 171 | 19017 | 7 |
| `reservation-length` | `harness` | Failed | 4 of 5 | 106 | 4259 | 2 |
| `reservation-length` | `bare` | Failed | 4 of 5 | 36 | 3651 | 5 |

Two of the eight runs passed, one in each arm. What the round showed:

- **With the harness, the agent kept what `make check` holds.** All four runs with the harness ran `make check` and ended with it passing. Without the harness, two of the four runs ran it, and the run of `time-zone-in-mail` ended with a finding of `lint-i18n`.
- **Neither arm kept what `make check` does not hold.** The six failed runs failed on three things, and each task failed on the same thing in both arms.
  - `room-floor`, an ignore comment. Both agents gave the floor the type `int`, converted it to `int32` for the database, and silenced gosec with `//nolint:gosec` and a reason. `AGENTS.md` forbids an ignore comment, and the code of the repository had seven at the time, four of them at a conversion of this shape. The agent with the harness had the rule, and followed the code. The solution of the task needs no conversion: it keeps the `int32` that the API document gives.
  - `time-zone-in-mail`, an integration test. A test of the relay compares the whole data of a published event, and the new property made it fail. The test runs against PostgreSQL in `make check-all`, not in `make check`.
  - `reservation-length`, a scenario in a browser. The guide and the skill for a screen ask for a scenario in `frontend/e2e/`, and no check does. Both agents wrote the unit test of the pure logic and stopped there.

  This is the other side of what TPL-0020 argues. A rule that a fast check holds was kept. A rule that a sentence, a guide, or a slow check holds was not, with the harness or without it.
- **The slow checks did not fit into a run.** With the harness, the agent started `make check-all` in the background in three of the four tasks. Twice, the limit of time of one command ended it before it finished, and the agent said so in its report. Once it finished, with three failures that came from the copy, whose cache has no Terraform provider. The wait made the runs with the harness two to nine times as long as the runs without it. In `time-zone-in-mail`, `make test-integration` alone would have shown the failing test in time.
- **Fewer commands were refused with the harness:** 2, and 17 without it. Without the harness, the agent reached for `npx`, for `python3`, and for chains of `cd` and `cat`. With it, the agent used the make targets.
- **Nothing else was weakened.** No run touched an allowance file, a hook, or a check.
- **No instruction was added.** With the harness, three criteria failed, once each. Each asks for a check or for a change of the code before it asks for a sentence, and one run is no ground for either.

### What the runs cannot show

The runs cannot show how often each arm fails, whether a difference holds for another model, and what the harness costs. In the second round, each arm passed one task of four. With one run of each, that is not a measurement either.

### Faults of the evaluation that the runs showed

- **A variable reached the copy.** One run before the first round is not recorded. `make eval TASK=...` passed its variable `TASK` on to every `make` in the copy, where the make files use that name, so `make check` failed for the agent and for the grader. The agent found the cause and reported it. The variables are `EVAL_TASK` and `EVAL_ARM` since then, and the script drops what the calling make exports.
- **A run that the usage limit ended was recorded.** The first attempt at the second round is not recorded. The usage limit of the account ended its first run, and the script wrote all eight runs as failures of five seconds. Since then, a run that the usage limit or an error of the API ended writes no record, and `make eval` fails.
- **The turns of a run were those of its last stretch.** Claude Code counts the turns and the usage from zero again each time a command in the background wakes the agent: a run of 33 minutes had 2 turns. The output tokens of every record are now those of the whole run. The turns of a record are still those of the last stretch, so the table of the second round leaves them out.
- **`make check` could not pass in the bare arm.** Two checks failed there whatever the agent did: `lint-links`, because `README.md` links to the `AGENTS.md` that the arm removes, and `test-hooks`, because the hooks are gone. The agent of `rename-room` saw both and judged them rightly. Another agent may have given up on `make check`. Since then, the arm also removes the tests of the hooks and the line of the README that lists the instruction files, and `make test-evals` proves that `make check` passes in a bare copy.
- **A check that was killed left the copy broken.** In one copy, the checks of Terraform failed, and failed again when the agent ran them again: "there is no package ... cached in .terraform/providers". The copy was not the cause, and any checkout could meet the fault. The limit of time of one command had ended `make check-all` while Terragrunt initialized its units, and Terragrunt trusted the half-initialized directories from then on. Since then, a run that does not end well leaves a mark, and the next run starts from clean working data (`tools/scripts/infra.sh`). One run with the harness also left a lock file of Terraform, which `make check-all` had changed, in its change.

## What to do next

- Run each arm of each task at least five times before reading a rate out of the table.
- The rule on ignore comments and the code agree since the round, and a check holds the rule. Capacity, the limit of a page, and attendees keep the `int32` of the API document, so the code that an agent copies has no conversion, and the scripts pass a list through `xargs` or as an array. Four ignore comments are left, each where the check is wrong about the code. `ignore-comments.allow` names them with their reasons, and `make check` fails on any other (`lint-ignore-comments`).
- Give the agent a slow check that fits into a run. The guide and the skill for events name `make test-integration` since the round, the part of `make check-all` that a change to an event needs. The other guides still name `make check-all`.
- Decide whether a small change to a screen that exists needs a scenario in a browser. The guide says so, and nothing checks it.
- Consider a criterion for `booking-horizon` that the tests hold the new rule: `make test-mutation`, which `make check` does not run.
