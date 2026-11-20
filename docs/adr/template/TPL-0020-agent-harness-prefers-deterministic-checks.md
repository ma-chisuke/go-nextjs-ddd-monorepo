---
status: accepted
date: 2026-11-20
---

# TPL-0020: Prefer deterministic checks in the harness for coding agents, and start with minimal instructions

## Context and Problem Statement

Coding agents write much of the code and many of the documents in this template, and in the products that teams make from it.
An agent can break a rule that exists only in prose: it may not read the rule, may lose it during a long task, or may misread it.
Nothing then tells the agent, or anyone else, that the rule was broken.

The usual answer is an instruction file that describes the repository: its architecture, its conventions, and every rule.
An agent reads such a file on every task, so the file costs tokens every time. It goes stale as the code changes, and it gives no signal when an agent breaks a rule in it.
A 2026 study from ETH Zurich, "Evaluating AGENTS.md: Are Repository-Level Context Files Helpful for Coding Agents?", reported that repository overview files did not raise the rate at which agents completed tasks, and raised the cost by more than 20 percent.

How do we make agents follow the rules of this template, and what belongs in the instructions that every agent reads?

## Decision Drivers

- A rule that matters holds even when an agent did not read it or did not understand it.
- An agent can correct its own mistake from the output of a check, without a person.
- Every agent reads the instructions on every task, so each line must be worth its cost.
- The same rules hold for every agent product and for people.
- A change to the instructions rests on evidence of a failure, not on a guess.

## Considered Options

- Deterministic checks first, and minimal instructions that grow only from evaluated failures
- A long instruction file that describes the architecture and every rule
- No harness: agents work from the code and the README alone
- A separate rule file for each agent product

## Decision Outcome

Chosen option: "Deterministic checks first, and minimal instructions that grow only from evaluated failures", because a check holds a rule whatever the agent read, and a short instruction file costs little and stays true.

The rules are as follows.

- A rule that matters is enforced by a check. A finding is one line that gives the file and the line, the rule, how to fix it, and the record that holds the reason, so an agent can correct itself from that line.
- `AGENTS.md` holds only the commands, where things go, the instruction "run `make check` before you finish", and the rules that differ from common defaults. Reasons live in the records, and procedures live in skills and guides.
- `CLAUDE.md` only imports `AGENTS.md`, so every agent reads the same file. `backend/` and `frontend/` each have an `AGENTS.md` of their own, with a `CLAUDE.md` that imports it, for the instructions of that part.
- No tool writes into an `AGENTS.md`. Next.js 16 adds a block of its own to the `AGENTS.md` of an app when `next dev` sees a coding agent; that block would fail the prose checks of this repository, and would come back after every removal. `frontend/next.config.ts` turns it off (`agentRules: false`), and `frontend/AGENTS.md` says what the block says: Next.js 16 differs from what an agent knows, so an agent reads the documentation bundled with the installed version, under `frontend/node_modules/next/dist/docs/`, before it writes code.
- An instruction is added to `AGENTS.md` only after the evaluation in `evals/` shows a failure that no check can catch. The pull request that adds it names the records of the runs that showed the failure.

The harness has the following parts.

- Hooks for Claude Code. They format a file after an agent edits it. They block edits to generated files, reading `.env`, applying changes to prd, and force pushes. At the end of a turn, they run the checks for what changed and hand failures back to the agent, a bounded number of times.
- Skills for recurring procedures, in `.claude/skills/`: `write-adr` writes a record, and `sync-translation` brings a translation up to date. The skills of the code, such as `add-endpoint` and `add-screen`, each follow a guide in `docs/guides/`, and arrive with the code that they change.
- Subagents that bring eyes other than the author's, in `.claude/agents/`: `design-reviewer` reviews a change against the records, and `translation-reviewer` compares a translation with its source.
- Permissions that let safe commands run without asking and deny dangerous ones.
- lefthook, which gives people the same checks before a commit and before a push (`lefthook.yml`).

The evaluation in `evals/` measures the harness, and works as follows.

- A run gives a coding agent a task of this repository in one of two arms: `harness`, the repository as it is, or `bare`, the repository without `AGENTS.md`, `CLAUDE.md`, and `.claude/`. Everything else is the same in both arms, the checks included.
- Each run happens in a copy outside the checkout, a Git repository of its own without `evals/`, so the agent can change nothing in this repository and finds no solution of the task.
- A grader decides each run by the criteria of the task and by two criteria that every change must meet: `make check` passes, and the change weakens no check.
- `make test-evals`, in `make check-all`, proves the graders without an agent. `make eval` starts an agent, and only a person runs it, because a run costs usage and time.

The first runs, on November 20, 2026, gave each of the two tasks once to each arm, with the model `sonnet`, and three of the four runs passed every criterion.
Without the harness, the agent did the work of the task `booking-horizon` and never ran `make check`, which then reported five findings: generated code that was stale in two files, a Go file that was not formatted, and two sentences of a translation. With the harness, the agent ran `make gen` and `make check` before it stopped.
In the task `read-the-logs`, both arms updated the translation; without the harness, the agent computed the marker of the translation by hand.
No criterion failed with the harness, so no instruction was added. Four runs prove nothing: they are a first look, and the records of the evaluation, `evals/results/README.md`, say what they cannot show.

### Consequences

- Good, because a rule holds whether or not the agent read about it, and the agent learns the rule at the moment it breaks it.
- Good, because people, agents, and CI share one `make check`, so no agent product needs rules of its own.
- Good, because `AGENTS.md` stays short, so it costs little on every task and has little that can go stale.
- Good, because a new instruction rests on runs, not on a guess: the evaluation gives the same task to an agent with the harness and without it, under the same checks.
- Bad, because some rules cannot be checked, such as naming, the shape of a domain model, or whether a test tests the right thing. They depend on review, and on the subagent that reviews.
- Bad, because few findings name their record: those of the checks that this repository writes itself do, and so do the findings of Trivy on licenses. The other findings, including the ones that this repository reformats into one line (gitleaks, Trivy, ESLint, tflint, and others), give the rule and a fix or a link, and the agent has to find the reason from the name of the rule.
- Bad, because the evaluation is small. Two tasks cover two kinds of change, and four runs are a first look, so whether the short `AGENTS.md` is enough is not measured yet; the choice still rests on the study and on judgment.
- Bad, because the hooks work only in Claude Code. Other agents get the checks through `make check`, lefthook, and CI alone.
- Bad, because the evaluation runs Claude Code only.
- Bad, because a run spends the usage of the account of whoever starts it, and takes between a few minutes and half an hour, so no check starts it, and nobody is made to run it.

### Confirmation

- `make check` is the gate. People, agents, and CI run it, and the hooks run it at the end of a turn.
- `make check` runs the target `lint-agents-md`, which keeps `AGENTS.md` short.
- The hooks and the permissions are registered in `.claude/settings.json`, and the hooks live in `.claude/hooks/`. `make check` feeds each hook sample events and checks what it blocks and allows (`test-hooks`).
- `make check-all` runs `test-evals`, which proves the graders of the evaluation without an agent: for each task, the repository as it is fails, and the solution of the task passes every criterion, `make check` included.
- No check runs `make eval`, so review confirms that a new instruction in `AGENTS.md` comes with the records of the runs that showed its failure, and that no check can replace it.

## Pros and Cons of the Options

### Deterministic checks first, and minimal instructions

- Good, because a check is the same for every agent, every model, and every person.
- Good, because the output of a failed check tells the agent what to fix, so it can fix the problem without a person.
- Bad, because writing a check, and keeping its output useful, costs more than writing a sentence.
- Bad, because an agent may miss context that a long file would have given it, and spend a turn on a failed check to learn it.

### A long instruction file

- Good, because everything is in one place, and a new agent or person reads it once.
- Bad, because every agent reads it on every task, and the study cited in this record found that such overviews raised the cost without raising success.
- Bad, because nothing tells the agent when it breaks a rule in the file, and nothing tells anyone when the file goes stale.

### No harness

- Good, because there is nothing to build or maintain.
- Bad, because each agent rediscovers the commands and the rules, breaks the ones it does not find, and leaves those to the reviewer.

### A separate rule file for each agent product

- Good, because each file can use the features of its product.
- Bad, because the files drift apart, and a rule changed in one file stays unchanged in the others.

## More Information

- [Evaluating AGENTS.md: Are Repository-Level Context Files Helpful for Coding Agents?](https://arxiv.org/abs/2602.11988), Thibaud Gloaguen and others, ETH Zurich and LogicStar.ai, 2026.
- [AGENTS.md](https://agents.md/)
- The tools behind `make check` are chosen in TPL-0023.
- `evals/README.md` describes the arms, the tasks, and how to run and read the evaluation. No document outside `evals/` links into it: a run removes the directory from the copy that the agent works in, and a link would be broken there.
