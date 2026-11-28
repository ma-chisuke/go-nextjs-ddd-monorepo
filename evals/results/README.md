# The records of the evaluation

Each file `<time>-<task>-<arm>.json` in this directory is one run of `make eval`: the task, the arm, the commit, the model, the time, what the agent used, and each criterion with its result. `make eval-report` prints them as a table.

## What the runs so far showed

Twenty runs were made on November 20 and 22, 2026, with the model `sonnet`, in three rounds. The first two rounds gave each of the six tasks once to each arm. The third round gave the four larger tasks once more to each arm, after the repairs that the second round led to.

Twenty runs prove nothing: an agent does not repeat itself, and one run in each arm cannot tell a difference from chance. They are a first look. They show that the evaluation works from end to end, and where to look next.

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
| `rename-room` | `harness` | Passed | 7 of 7 | 1998 | 19422 | Not counted |
| `rename-room` | `bare` | Passed | 7 of 7 | 547 | 14005 | 5 |
| `room-floor` | `harness` | Failed | 5 of 6 | 2480 | 15077 | Not counted |
| `room-floor` | `bare` | Failed | 5 of 6 | 1137 | 19524 | Not counted |
| `time-zone-in-mail` | `harness` | Failed | 5 of 6 | 1506 | 17466 | Not counted |
| `time-zone-in-mail` | `bare` | Failed | 4 of 6 | 171 | 19017 | 7 |
| `reservation-length` | `harness` | Failed | 4 of 5 | 106 | 4259 | 2 |
| `reservation-length` | `bare` | Failed | 4 of 5 | 36 | 3651 | 5 |

"Not counted" marks a run that a command in the background woke: when it ran, the script counted the refused commands of the last stretch alone, which was 0 in each of these runs. Its record holds `null`.

Two of the eight runs passed, one in each arm. What the round showed:

- **With the harness, the agent kept what `make check` holds.** All four runs with the harness ran `make check` and ended with it passing. Without the harness, two of the four runs ran it, and the run of `time-zone-in-mail` ended with a finding of `lint-i18n`.
- **Neither arm kept what `make check` does not hold.** The six failed runs failed on three things, and each task failed on the same thing in both arms.
  - `room-floor`, an ignore comment. Both agents gave the floor the type `int`, converted it to `int32` for the database, and silenced gosec with `//nolint:gosec` and a reason. `AGENTS.md` forbids an ignore comment, and the code of the repository had seven at the time, four of them at a conversion of this shape. The agent with the harness had the rule, and followed the code. The solution of the task needs no conversion: it keeps the `int32` that the API document gives.
  - `time-zone-in-mail`, an integration test. A test of the relay compares the whole data of a published event, and the new property made it fail. The test runs against PostgreSQL in `make check-all`, not in `make check`.
  - `reservation-length`, a scenario in a browser. The guide and the skill for a screen ask for a scenario in `frontend/e2e/`, and no check does. Both agents wrote the unit test of the pure logic and stopped there.

  This is the other side of what TPL-0020 argues. A rule that a fast check holds was kept. A rule that a sentence, a guide, or a slow check holds was not, with the harness or without it.
- **The slow checks did not fit into a run.** With the harness, the agent started `make check-all` in the background in three of the four tasks. Twice, the limit of time of one command ended it before it finished, and the agent said so in its report. Once it finished, with three failures that came from the copy, whose cache has no Terraform provider. The wait made the runs with the harness two to nine times as long as the runs without it. In `time-zone-in-mail`, `make test-integration` alone would have shown the failing test in time.
- **The refused commands cannot be compared.** The records give 2 with the harness and 17 without it, and an earlier version of this page read a difference out of that. Three of the four runs with the harness, and one without it, count the last stretch alone. Without the harness, the agent reached for `npx`, for `python3`, and for chains of `cd` and `cat`.
- **Nothing else was weakened.** No run touched an allowance file, a hook, or a check.
- **No instruction was added.** With the harness, three criteria failed, once each. Each asks for a check or for a change of the code before it asks for a sentence, and one run is no ground for either.

### The third round: the four larger tasks again

The third round, on November 22, used Claude Code 2.1.330. Its commit has the repairs that the second round led to: the code keeps the `int32` of the API document, `lint-ignore-comments` fails on a new ignore comment, the guide for events names `make test-integration`, and `make check` passes in the bare arm.

| Task | Arm | Result | Criteria passed | Seconds | Output tokens | Refused commands |
|---|---|---|---|---|---|---|
| `rename-room` | `harness` | Passed | 7 of 7 | 250 | 13411 | 9 |
| `rename-room` | `bare` | Passed | 7 of 7 | 515 | 13598 | 6 |
| `room-floor` | `harness` | Passed | 6 of 6 | 1654 | 14323 | Not counted |
| `room-floor` | `bare` | Failed | 5 of 6 | 2791 | 22088 | Not counted |
| `time-zone-in-mail` | `harness` | Passed | 6 of 6 | 1158 | 20827 | 9 |
| `time-zone-in-mail` | `bare` | Failed | 4 of 6 | 166 | 19039 | 9 |
| `reservation-length` | `harness` | Failed | 4 of 5 | 215 | 7603 | 5 |
| `reservation-length` | `bare` | Passed | 5 of 5 | 111 | 6352 | 4 |

Five of the eight runs passed: three with the harness, and two without it. The machine did other work during the whole round, with a load average between 9 and 19, so the seconds say little, and they cannot be compared with those of the second round. What the round showed:

- **With the harness, every run ended with `make check` passing, as in the rounds before.** Without the harness, two of the four runs did. The other two ended with a finding of `lint-i18n`: the agent changed a document and its translation, and left the marker of the translation as it was.
- **No run wrote an ignore comment.** In `room-floor`, both agents kept the `int32` of the API document from the handler to the database, as the code around the change now does. The run with the harness passed. So the change of the code was enough here, and the round does not show `lint-ignore-comments` at work. The run without the harness was stopped at the limit of time of a run, 2700 seconds, before it had brought the markers of two translations up to date.
- **With the harness, the agent ran the integration tests.** In `time-zone-in-mail`, it ran `make test-integration`, which the guide for events names since the second round, changed the test of the relay that compares the whole data of an event, and passed. Without the harness, the agent changed another integration test, said that it had run none, and failed on the test of the relay, as in the second round.
- **The scenario in a browser is still left to chance.** In `reservation-length`, the agent with the harness wrote the unit test and no scenario, as both agents did in the second round. The agent without the harness added a line to a scenario that exists, and passed. One run of four wrote a scenario, and it had no harness.
- **A slow check fits into a run when it is a part of `make check-all`.** `make test-integration` ended within the time limit of a command. In `room-floor`, the agent with the harness started `make check-all` in the background, the time limit ended it, and the agent said so in its report. In `rename-room`, it did not start the slow checks, and said so.
- **The refused commands did not differ.** In the six runs that count them in full, the agent with the harness was refused 9, 9, and 5 commands, and the agent without it 6, 9, and 4. Both reached for `python3`, for `sed -i`, and for chains of `cd` and `cat`.
- **Nothing was weakened.** No run touched an allowance file, a hook, or a check.
- **No instruction was added.** With the harness, one criterion failed, the scenario. Whether a small change to a screen needs one is not decided yet, and the next section lists it.

### What the runs cannot show

The runs cannot show how often each arm fails, whether a difference holds for another model, and what the harness costs. In the second round, each arm passed one task of four. In the third, the arm with the harness passed three, and the arm without it two. The repairs between the rounds, another version of Claude Code, and chance all changed at once, and with one run of each, neither round is a measurement.

### Faults of the evaluation that the runs showed

- **A variable reached the copy.** One run before the first round is not recorded. `make eval TASK=...` passed its variable `TASK` on to every `make` in the copy, where the make files use that name, so `make check` failed for the agent and for the grader. The agent found the cause and reported it. The variables are `EVAL_TASK` and `EVAL_ARM` since then, and the script drops what the calling make exports.
- **A run that the usage limit ended was recorded.** The first attempt at the second round is not recorded. The usage limit of the account ended its first run, and the script wrote all eight runs as failures of five seconds. Since then, a run that the usage limit or an error of the API ended writes no record, and `make eval` fails.
- **The turns of a run were those of its last stretch.** Claude Code counts the turns and the usage from zero again each time a command in the background wakes the agent: a run of 33 minutes had 2 turns. The output tokens of every record are now those of the whole run. The tables of the second and third rounds leave the turns out, because the records of that time held those of the last stretch.
- **The refused commands of a run were those of its last stretch too.** The third round showed it: every record of a run that a command in the background woke has no refused command, and the other records of the larger tasks have two to nine. The second round has four such records, three of them with the harness, and this page first read "fewer commands were refused with the harness" out of them. The tables now mark those runs. Since November 28, the script reads the stream of a run, which holds one result for each stretch, and adds up the turns and the refused commands. `make test-evals` proves the sum on a stream of two stretches, and one run of `read-the-logs`, which is not recorded, proved it with an agent. The six earlier records of runs with more than one stretch hold `null` for both.
- **`make check` could not pass in the bare arm.** Two checks failed there whatever the agent did: `lint-links`, because `README.md` links to the `AGENTS.md` that the arm removes, and `test-hooks`, because the hooks are gone. The agent of `rename-room` saw both and judged them rightly. Another agent may have given up on `make check`. Since then, the arm also removes the tests of the hooks and the line of the README that lists the instruction files, and `make test-evals` proves that `make check` passes in a bare copy.
- **A check that was killed left the copy broken.** In one copy, the checks of Terraform failed, and failed again when the agent ran them again: "there is no package ... cached in .terraform/providers". The copy was not the cause, and any checkout could meet the fault. The limit of time of one command had ended `make check-all` while Terragrunt initialized its units, and Terragrunt trusted the half-initialized directories from then on. Since then, a run that does not end well leaves a mark, and the next run starts from clean working data (`tools/scripts/infra.sh`).
- **A check that was killed left a file in the change.** In both runs of `room-floor` in the third round, and in two runs with the harness in the second, the change of the agent held a lock file of Terraform in a module, `infra/modules/<module>/.terraform.lock.hcl`. `validate-terraform` lets `terraform init` write that file and removes it afterwards. The time limit ended `make check-all` in between, and every later run took the file for the module's own and kept it. Since then, the script marks a lock file that it will remove, and the next run removes what a killed run left (`tools/scripts/terraform-validate.sh`).
- **The machine was not idle.** During the third round, tests of another project ran on the machine. `make check` took 282 seconds in the grading of one run, and takes about 30 on an idle machine. The run of `room-floor` without the harness reached the limit of time of a run, and whether it would have finished on an idle machine is not known.

## What to do next

- Run each arm of each task at least five times before reading a rate out of the table.
- The rule on ignore comments and the code agree since the second round, and a check holds the rule. Capacity, the limit of a page, and attendees keep the `int32` of the API document, so the code that an agent copies has no conversion, and the scripts pass a list through `xargs` or as an array. Four ignore comments are left, each where the check is wrong about the code. `ignore-comments.allow` names them with their reasons, and `make check` fails on any other (`lint-ignore-comments`).
- Give the agent a slow check that fits into a run. The guide and the skill for events name `make test-integration` since the second round, the part of `make check-all` that a change to an event needs, and in the third round the agent ran it. Since the third round, the guides and the skills for an endpoint, a schema, and a context name their parts too, each measured at five minutes or less, and `AGENTS.md` says that `make check-all` takes ten minutes or more. No run has tried them yet.
- Decide whether a small change to a screen that exists needs a scenario in a browser. The guide says so, and nothing checks it: one run of four wrote one.
- Consider a criterion for `booking-horizon` that the tests hold the new rule: `make test-mutation`, which `make check` does not run.
