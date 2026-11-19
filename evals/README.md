# Evaluate the agent harness

The harness of this repository is what a coding agent reads and runs into beyond the code: the `AGENTS.md` files, and the hooks, skills, and subagents in `.claude/`.
TPL-0020 keeps it small on purpose, and allows a new instruction only after an evaluation shows a failure that no check can catch.
This directory is that evaluation: an agent does a task of this repository twice, once with the harness and once without it, and a grader says what each result is worth.

## What a run compares

| Arm | What the agent gets |
|---|---|
| `harness` | The repository as it is |
| `bare` | The repository without `AGENTS.md`, `CLAUDE.md`, and `.claude/` |

Everything else is the same in both arms: the code, the guides, the checks, the Git hooks, the model, the limits, and the tools that the agent may use without asking (`allowed-tools.txt`).
`make check` exists in both arms. Only the harness tells the agent to run it, and only the harness stops a turn that ends with a failing check.

A run copies the files of one commit into a directory outside the checkout and makes a Git repository of its own there, so the agent cannot change this repository, and no instruction file of a parent directory reaches it. The copy has no `evals/`: an agent that found a solution there would not have to do the task.
Settings and memory of the user are left out, so a run does not depend on whose machine it is.
After a run of the bare arm, the files of the harness come back before the grading, so both arms are graded as the same repository.

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

Every task also has two criteria that hold for every change of this repository: `make check` passes, and the change weakens no check. It touches no allowance or ignore file, no hook, and no check, and it adds no ignore comment.

## Run it

```sh
make eval TASK=read-the-logs ARM=harness
make eval TASK=read-the-logs ARM=bare
make eval-report
```

`make eval` starts Claude Code without a terminal (`claude -p`), which spends the usage of the account of whoever runs it, and takes between a few minutes and half an hour for one run. No check starts it.
It writes a record to `results/`: the task, the arm, the commit, the model, the turns and the time, the commands that were refused, and each criterion with its result. The change of the agent itself stays outside the repository; the last line of the run says where.

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
4. Do the task yourself, and save the change as `solution.patch` (`git diff > evals/tasks/<task>/solution.patch`).
5. Run `make test-evals`. It fails when the repository as it is already passes the criteria of the task, when the solution does not apply any more, or when the solution fails a criterion.

`make test-evals` is part of `make check-all`, and runs when a file of `evals/` or a file that a solution changes is in scope. A change to the code under a solution therefore fails until the solution follows it.

## The records so far

`make eval-report` prints the table. [results/README.md](results/README.md) says what the runs so far showed, and what they are too few to show.
