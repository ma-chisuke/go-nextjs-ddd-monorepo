# Mutation tests

Coverage tells which lines the tests run. It does not tell whether a test would fail if a line were wrong.
Mutation testing answers that question: a tool changes the code in one small way, such as `<` into `<=`, `true` into `false`, or a statement removed, and runs the tests against the changed code.
Each change is a mutant. A test that fails kills the mutant. A mutant that every test passes lives, and shows a rule that the tests do not hold.

## What test-mutation checks

`make test-mutation` runs gomutants, which `tools/go.mod` pins, on the packages that `MUTATION_PACKAGES` lists in `make/go.mk`: the domain and the use cases of each core context, where the rules of the business live (TPL-0003).
For Reservation these rules are the overlap of periods, the capacity of a room, the grid of 15 minutes, the longest period, the past, and the notice before a cancellation.
The supporting contexts are not mutated. Their use cases are transaction scripts whose tests run against a database, so a run would be slow and would say little.

- gomutants changes the code through an overlay of `go test`, so it never writes to the source files.
- A mutant runs only the tests of its own package that cover the changed line. The tests of the domain must therefore hold the rules of the domain themselves; an API test does not count.
- Every mutant that lives fails the check, and so does every mutant on a line that no test runs. There is no percentage to reach: a threshold such as 80 percent would let a rule slip through unseen.
- A mutant that makes a test hang counts as killed, because a test noticed it. A mutant that does not compile does not count.
- A mutant that compiles to the same machine code as the code is equivalent, and gomutants drops it by itself (`--detect-equivalent`).

The check is part of `make check-all`, and runs when a Go file of a listed package, `backend/go.mod`, `equivalent-mutants.allow`, or the definition of the check is in scope.
In November 2026, a run over the domain and the use cases of Reservation made 338 mutants and took about 100 seconds on a laptop busy with other work, with 4 mutants at a time.
`MUTATION_WORKERS` sets how many mutants run at once; by default, half of the processors.

## Run it for one package

```sh
make test-mutation MUTATION_PACKAGES=backend/internal/reservation/internal/domain
```

The report of gomutants, with every mutant and its status, stays in `.cache/mutation/backend.json`.

When you add a core context, add its `internal/domain` and `internal/app` to `MUTATION_PACKAGES`.

## Read a finding

A mutant that lives gives a finding such as this one:

```text
backend/internal/reservation/internal/domain/reservation.go:168: [mutation/survived] every test passes with `<` changed to `<=` (CONDITIONALS_BOUNDARY, mutant backend/internal/reservation/internal/domain/reservation.go:checkTerms:CONDITIONALS_BOUNDARY#2). ...
```

The line is `attendees < 1`, in the rule that a reservation has at least one attendee. No test booked a room for exactly one attendee, so a rule of "at least two" would pass every test.

The name of a mutant is `<file>:<function>:<type>#<n>`: the n-th place of that type in that function. A change elsewhere in the file leaves the name as it is; a change in the same function can renumber it.
`[mutation/not-covered]` names a line that no test runs at all.

To run one mutant alone, with its verdict:

```sh
make mutant ID='backend/internal/reservation/internal/domain/reservation.go:checkTerms:CONDITIONALS_BOUNDARY#2'
```

It prints `KILLED` or `LIVED`, and fails when the mutant lives.
To see the mutant fail or pass a test by hand, make the change of the finding at its line, run `go test` on the package, and undo the change.

## Fix a survivor

A mutant that lives asks one of two questions.

- Does a test check what this code does? Usually not, and the fix is a test that fails with the change. Write it in the style of the tests of the package, often as a new row of a table test, and name the rule that it holds. For the finding above, the fix is a booking for the organizer alone.
- Is the code needed? A condition that a later check repeats, or a branch that cannot happen, makes a mutant that no test can kill. Make the code simpler, in a commit of its own. For example, a use case once tested `found || err != nil` while every error came with `found` set; it now tests one flag.

A test that only executes a line kills nothing: assert the result that the change would alter.
If the survivor shows that the code is wrong, not only untested, fix the code, and say so in the commit.

## When an allowance is right

Some mutants are equivalent: no test can tell them from the code, because the change cannot alter what the code does.
Prefer to change the code until the mutant is gone. For example, the opening and the closing of a room were each built with their own seconds, and moving only one of them by one second changed nothing, because periods lie on the grid of 15 minutes. Both now come from one function, so the change moves both, and the tests of the bounds notice it.

When the code cannot change, `equivalent-mutants.allow` at the root lets the mutant live, one line per mutant:

```text
# <mutant> | <original> -> <replacement> | <reason>
```

- The mutant and the change are written as the finding names them.
- The reason proves that the change is equivalent. "Hard to test" is not a proof: a mutant that a test could kill is a test to write.
- Adding a line is a decision of review. An agent stops and asks, with the finding and the proof, instead of writing the line.
- A line that matches no living mutant fails the check (`mutation/unused-allowance`), and so does a line without its reason (`mutation/allowance-format`). When a test kills the mutant, or the code changes, the line has to go, so the file does not rot.
