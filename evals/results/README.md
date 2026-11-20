# The records of the evaluation

Each file `<time>-<task>-<arm>.json` in this directory is one run of `make eval`: the task, the arm, the commit, the model, the turns and the time, and each criterion with its result. `make eval-report` prints them as a table.

## What the runs so far showed

The first runs were made on November 20, 2026: each task once in each arm, four runs, with the model `sonnet` (Claude Code 2.1.327).

| Task | Arm | Result | Criteria passed | Turns | Seconds | Refused commands |
|---|---|---|---|---|---|---|
| `read-the-logs` | `harness` | Passed | 5 of 5 | 8 | 105 | 1 |
| `read-the-logs` | `bare` | Passed | 5 of 5 | 10 | 25 | 2 |
| `booking-horizon` | `harness` | Passed | 7 of 7 | 50 | 240 | 3 |
| `booking-horizon` | `bare` | Failed | 6 of 7 | 52 | 130 | 10 |

Four runs prove nothing: an agent does not repeat itself, and one run in each arm cannot tell a difference from chance. They are a first look, and they show that the evaluation works from end to end. What they showed:

- **Without the harness, the agent did the work and did not finish it.** In `booking-horizon`, both arms wrote the rule in the domain with its test, the type of problem, its page in both languages, the README of the context, and the words on the screen, and both described the rule in the API document, which the task did not ask for. Without the harness, the agent stopped there: the code generated from the API document was stale in two files, a Go file was not formatted, and two sentences of a translation broke the rules of the Japanese prose. `make check` reports all five; the agent never ran it. With the harness, the agent ran `make gen` and `make check` before it stopped. This is the case that TPL-0020 argues from: the checks hold the rules, and the harness has little more to do than to make the agent run them.
- **The translation followed in both arms.** In `read-the-logs`, the agent without the harness also wrote the Japanese section, and brought the marker of the translation up to date by computing the hash itself, since nothing told it of `make i18n-stamp`. The guides of the repository stay in the bare arm, and they were enough here.
- **Nothing was weakened.** No run touched an allowance file, a hook, or a check, or added an ignore comment.
- **No instruction is missing yet.** No criterion failed with the harness, so these runs give no reason to add a sentence to `AGENTS.md`.

What these runs cannot show: how often each arm fails, whether the difference holds for another model, and what the harness costs. The harness arm took longer in both tasks, because it ran the checks; four runs are too few to put a number on that.

One earlier run is not recorded. It showed a fault of the evaluation itself: `make eval TASK=...` passed its variable `TASK` on to every `make` in the copy, where the make files use that name, so `make check` failed for the agent and for the grader. The agent found the cause and reported it. The variables are `EVAL_TASK` and `EVAL_ARM` since then, and the script drops what the calling make exports.

## What to do next

- Run each arm of each task at least five times before reading a rate out of the table.
- Add tasks for the other kinds of change that the guides describe: an endpoint, a schema change, an event, a screen.
- Consider a criterion for `booking-horizon` that the tests hold the new rule: `make test-mutation`, which `make check` does not run.
