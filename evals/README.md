# Evaluate the agent harness

The harness of this repository is what a coding agent reads and runs into beyond the code: the `AGENTS.md` files, and the hooks, skills, and subagents in `.claude/`.
TPL-0020 keeps it small on purpose, and allows a new instruction only after an evaluation shows a failure that no check can catch.
This directory is that evaluation: an agent does a task of this repository twice, once with the harness and once without it, and a grader says what each result is worth.

## What a run compares

| Arm | What the agent gets |
|---|---|
| `harness` | The repository as it is |
| `bare` | The repository without `AGENTS.md`, `CLAUDE.md`, and `.claude/`, and without what only makes sense with them: the tests of the hooks, and the line of the README that lists the instruction files |

Everything else is the same in both arms: the code, the guides, the checks, the Git hooks, the model, the limits, and the tools that the agent may use without asking (`allowed-tools.txt`).
`make check` exists in both arms. Only the harness tells the agent to run it, and only the harness stops a turn that ends with a failing check.

A run copies the files of one commit into a directory outside the checkout and makes a Git repository of its own there, so the agent cannot change this repository, and no instruction file of a parent directory reaches it. The copy has a branch `origin/main` at that commit, as a clone has, so the checks that compare a change with its base work there. The copy has no `evals/`: an agent that found a solution there would not have to do the task.
Settings and memory of the user are left out, so a run does not depend on whose machine it is.
`make check` passes in both arms before the agent works, so a check that fails after a run is the work of the agent.
After a run of the bare arm, the files of the harness come back before the grading, so both arms are graded with the same checks.

## The tasks

A task is a directory `tasks/<task>/`:

| File | Content |
|---|---|
| `task.txt` | What the agent is asked to do, as a person would ask: the goal, not the steps |
| `grade.sh` | The criteria of the task. Each one passes when its command succeeds in the copy |
| `solution.patch` | A change that passes every criterion. It proves that the task can be done, and that the grader can tell |

| Task | What it asks | What it measures |
|---|---|---|
| `read-the-logs` | A new section in a guide | The translation changes with the English text (TPL-0021) |
| `booking-horizon` | A business rule in the core context, with a new type of problem | The rule lives in the domain, and the pages, the README of the context, and the words on the screen follow it (TPL-0004, TPL-0008) |
| `rename-room` | An operation of the API of a supporting context: rename a room | The operation starts in the API document, and the generated code, the queries, both stores (the real one and the fake of the API tests), the API tests, and the README of the context follow it (TPL-0006, TPL-0007) |
| `room-floor` | An optional property of a room, kept in the database | The schema, the queries, the generated code, and the API document move together, and the change of the schema is one that a database with data can take (TPL-0010, TPL-0011) |
| `time-zone-in-mail` | A new property of an integration event, which its consumer reads | The contract between two contexts changes compatibly, on both sides, without a new version and without one context importing the other (TPL-0005, TPL-0013) |
| `reservation-length` | One more fact on a screen: the length of each reservation | The logic is pure and has a unit test, and the page that the server renders shows it |

Every task also has two criteria that hold for every change of this repository: `make check` passes, and the change weakens no check. It touches no allowance or ignore file, no hook, and no check, and it adds no ignore comment.

## Run it

```sh
make eval EVAL_TASK=read-the-logs EVAL_ARM=harness
make eval EVAL_TASK=read-the-logs EVAL_ARM=bare
make eval-report
```

`make eval` starts Claude Code without a terminal (`claude -p`), which spends the usage of the account of whoever runs it, and takes between a minute and three quarters of an hour for one run. No check starts it.
It writes a record to `results/`: the task, the arm, the commit, the model, the time, the output tokens, the commands that were refused, and each criterion with its result. The turns and the refused commands of a record are those of the whole run. Claude Code counts both from zero again each time a command in the background wakes the agent, so the script reads the stream of the run and adds up its stretches. Six records from before November 28, 2026 hold `null` for both: their runs had more than one stretch, and only the last one was counted. A run that the usage limit of the account or an error of the API ended writes no record, and `make eval` fails. The change of the agent itself stays outside the repository; the last line of the run says where.

| Variable | Meaning | Default |
|---|---|---|
| `EVAL_CLAUDE` | The command of Claude Code | `claude` |
| `EVAL_MODEL` | The model | `sonnet` |
| `EVAL_MAX_TURNS` | The limit of turns | `150` |
| `EVAL_TIMEOUT` | The limit of time, in seconds | `2700` |
| `EVAL_REF` | The commit to start from | `HEAD` |
| `EVAL_KEEP` | `1` keeps the copy after the run | not set |

One run says little: agents do not repeat themselves. Run each arm several times before you conclude anything, and commit the records, so that the next person sees what the decision rested on.

## Read a result

A criterion that fails in both arms is a task that the harness does not help with. Look at the change of the agent first: the criterion may ask for something that the task did not say.
A criterion that fails only without the harness is what the harness is for.
A criterion that fails with the harness, and that `make check` did not catch, is a candidate for a check. Only when no check can hold the rule does it become a sentence in `AGENTS.md`, with the records that showed the failure named in the pull request (TPL-0020).

## Add a task

1. Choose a change that a person of the team would ask for, and whose result can be judged by commands.
2. Write `task.txt` as that person would: what is wanted and why, without the list of files to touch. The list is what the evaluation measures.
3. Write `grade.sh` with a function `grade_task` that calls `criterion <name> <command>...`. Judge the behavior from outside, as the acceptance test of `booking-horizon` does through the API; a criterion that looks for a name that only one solution would choose fails good work.
   A grader needs no Docker of its own and no network. What the copy does not hold, it takes from the helpers that `run.sh` names under "What a grader may use": a database of its own on the local PostgreSQL, with the schema of the copy (`with_database`), a free port (`free_port`), the integration tests of a context (`integration_tests`), and the packages of Node installed in the copy (`own_dependencies`). A grader that starts a server stops it, also when it fails.
4. Do the task yourself, and save the change as `solution.patch`:

   ```sh
   git add --all && git diff --cached -- . ':!*.gen.go' ':!*.gen.ts' | sed -e '/^index /d' -e 's/^\(@@ [^@]* @@\).*/\1/' > evals/tasks/<task>/solution.patch
   ```

   The command leaves out the generated code, which `make test-evals` writes again with `make gen` after it applies the solution, so that a change to an API document does not break the solution of every task that touches the API. It also leaves out the lines that name the objects of Git, and the text after the line numbers of a hunk, which Git cuts off in the middle of a word: the spelling check would read both as words.
5. Run `make test-evals`, which starts the local PostgreSQL if it does not run. It first proves the bare arm: `make check` passes in a copy without the harness, and again once the harness is back. For each task, it fails when the repository as it is already passes the criteria of the task, when the solution does not apply any more, or when the solution fails a criterion. Break the solution in the way that each criterion is there to catch, and see the criterion fail.

`make test-evals` is part of `make check-all`, and runs when a file of `evals/` or a file that a solution changes is in scope. A change to the code under a solution therefore fails until the solution follows it.

No document outside `evals/` links into this directory; it is named by its path instead. A run removes the directory from the copy, where such a link would be broken and `make check` would fail for a reason that has nothing to do with the task. `make test-evals` fails on such a link, because the solutions are graded in the same kind of copy.

## The records so far

`make eval-report` prints the table. [results/README.md](results/README.md) says what the runs so far showed, and what they are too few to show.
