---
status: accepted
date: 2026-11-17
---

# TPL-0001: Record decisions as ADRs, and keep template decisions apart from project decisions

## Context and Problem Statement

This repository is a template.
A team copies it and then changes it into their own product.
A copy therefore holds two kinds of decisions: those that the template made before the copy existed, and those that the team makes afterward.

Most rules in this repository are enforced by a check, and a check can only say that a rule was broken.
A person or a coding agent who meets the rule also needs the reason for it, to decide whether to follow the rule or to change it.

How do we record decisions so that the reason behind a rule is easy to find, and so that a team can replace a template decision without losing what the template decided?

## Decision Drivers

- The message of a failed check must lead to the reason for the rule.
- A team must be able to take a later version of the template. If teams edit the records of the template, every update conflicts.
- A program must be able to check the records, because agents write many of them.
- The format must be one that people already know.

## Considered Options

- MADR 4.0.0, with template decisions and project decisions in separate directories
- MADR 4.0.0, with every decision in one directory and one sequence of numbers
- The original format by Michael Nygard
- No records: explain decisions in the README and in code comments

## Decision Outcome

Chosen option: "MADR 4.0.0, with template decisions and project decisions in separate directories", because it is the only option that keeps the decisions of the template stable while a team adds and replaces decisions.

The rules are as follows.

- A template decision is `docs/adr/template/TPL-NNNN-<slug>.md`. A project decision is `docs/adr/ADR-NNNN-<slug>.md`. Numbers start at 0001 and are never reused.
- A record starts with front matter that holds `status` (`proposed`, `accepted`, `rejected`, or `deprecated`) and `date` (the day of the last change to the decision).
- The title is `# <ID>: <title>`.
- A record has the sections "Context and Problem Statement", "Considered Options", and "Decision Outcome". "Decision Outcome" has the subsections "Consequences" and "Confirmation".
- "Confirmation" names the check that enforces the decision. If no check can enforce it, the section says that review enforces it.
- A team does not edit a template record. To decide otherwise, the team writes a project record whose front matter lists the replaced record, for example `supersedes: [TPL-0010]`.
- `docs/adr/README.md` lists every decision and marks the replaced ones. `make gen` writes it.
- Every record has a Japanese translation next to it (TPL-0021).

### Consequences

- Good, because a check message can cite an ID such as TPL-0004, and the ID leads to one file.
- Good, because a team can take a later version of the template: the files under `docs/adr/template/` are the same in the template and in the copy.
- Good, because the list shows which template decisions a team replaced, so a reader sees how far the copy has moved from the template.
- Bad, because a reader must look at the list, not at the template record, to learn whether a template decision is still in effect.
- Bad, because the template cannot quietly fix a published record. A change of meaning needs a new record.

### Confirmation

`make check` runs the target `lint-adr`, which fails in the following cases.

- A record has a wrong filename, front matter, title, or set of sections.
- `supersedes` names a record that does not exist.
- `docs/adr/README.md` differs from what `make gen` writes.
- A file under `docs/adr/template/` differs from the checksum in `docs/adr/template/checksums.txt`.

## Pros and Cons of the Options

### MADR 4.0.0, with template decisions and project decisions in separate directories

- Good, because the two directories show who owns a decision.
- Good, because the numbers of the template and of the project never collide when the template adds a record.
- Bad, because there are two sequences of numbers to learn.

### MADR 4.0.0, with every decision in one directory and one sequence of numbers

- Good, because there is one place to look.
- Bad, because a new template record and a new project record can take the same number.
- Bad, because nothing shows which records a team may edit.

### The original format by Michael Nygard

- Good, because it is short.
- Bad, because it has no place for the options that were rejected or for how the decision is confirmed, and those are what an agent needs before it proposes a change.

### No records

- Good, because there is nothing to maintain.
- Bad, because the reason for a rule is lost, and the next person either follows the rule blindly or removes it.

## More Information

- [MADR 4.0.0](https://adr.github.io/madr/)
- To write a record, use the skill in `.claude/skills/write-adr/`.
