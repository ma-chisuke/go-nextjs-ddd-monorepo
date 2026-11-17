---
status: accepted
date: 2026-11-17
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
- `CLAUDE.md` only imports `AGENTS.md`, so every agent reads the same file.
- An instruction is added to `AGENTS.md` only after an evaluation shows a failure that no check can catch. The evaluation lives in `evals/` and arrives in a later milestone.

The harness has the following parts.

- Hooks for Claude Code. They format a file after an agent edits it. They block edits to generated files, reading `.env`, applying changes to prd, and force pushes. At the end of a turn, they run the checks for what changed and hand failures back to the agent, a bounded number of times.
- Skills for recurring procedures, in `.claude/skills/`: `write-adr` writes a record, and `sync-translation` brings a translation up to date. More skills arrive with the code in later milestones.
- Subagents that bring eyes other than the author's, in `.claude/agents/`: `design-reviewer` reviews a change against the records, and `translation-reviewer` compares a translation with its source.
- Permissions that let safe commands run without asking and deny dangerous ones.
- lefthook, which gives people the same checks before a commit and before a push (`lefthook.yml`).

### Consequences

- Good, because a rule holds whether or not the agent read about it, and the agent learns the rule at the moment it breaks it.
- Good, because people, agents, and CI share one `make check`, so no agent product needs rules of its own.
- Good, because `AGENTS.md` stays short, so it costs little on every task and has little that can go stale.
- Bad, because some rules cannot be checked, such as naming, the shape of a domain model, or whether a test tests the right thing. They depend on review, and on the subagent that reviews.
- Bad, because few findings name their record: those of the checks that this repository writes itself do, and so do the findings of Trivy on licenses. The other findings, including the ones that this repository reformats into one line (gitleaks, Trivy, ESLint, tflint, and others), give the rule and a fix or a link, and the agent has to find the reason from the name of the rule.
- Bad, because until `evals/` exists, nothing measures whether the short `AGENTS.md` is enough. The choice rests on the study and on judgment.
- Bad, because the hooks work only in Claude Code. Other agents get the checks through `make check`, lefthook, and CI alone.

### Confirmation

- `make check` is the gate. People, agents, and CI run it, and the hooks run it at the end of a turn.
- `make check` runs the target `lint-agents-md`, which keeps `AGENTS.md` short.
- The hooks and the permissions are registered in `.claude/settings.json`, and the hooks live in `.claude/hooks/`. `make check` feeds each hook sample events and checks what it blocks and allows (`test-hooks`).
- The evaluation in `evals/` arrives in a later milestone. Until then, review confirms that a new instruction in `AGENTS.md` is one that no check can replace.

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
