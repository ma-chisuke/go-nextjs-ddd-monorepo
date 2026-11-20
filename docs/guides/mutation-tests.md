# Mutation tests

Coverage tells which lines the tests run. It does not tell whether a test would fail if a line were wrong.
Mutation testing answers that question: a tool changes the code in one small way, such as `<` into `<=`, `true` into `false`, or a statement removed, and runs the tests against the changed code.
Each change is a mutant. A test that fails kills the mutant. A mutant that every test passes lives, and shows a rule that the tests do not hold.

## What the mutation tests check

Two checks of `make check-all` run mutation tests, one for each language of code: `test-mutation` runs gomutants on the Go packages that hold the rules of the business, and `test-mutation-frontend` runs StrykerJS on the pure logic of the frontend.
Both follow the same rules.

- Every mutant that lives fails the check, and so does every mutant on a line that no test runs. There is no percentage to reach: a threshold such as 80 percent would let a rule slip through unseen.
- A mutant runs only the unit tests that run the changed line. An API test or a test in the browser does not count.
- A mutant that makes a test hang counts as killed, because a test noticed it. A mutant that does not compile does not count.
- Neither tool writes to the source files: gomutants changes the code through an overlay of `go test`, and StrykerJS changes a copy of the sources in `.cache/mutation/stryker/`.
- `MUTATION_WORKERS` sets how many mutants run at once; by default, half of the processors.

### Go: test-mutation

`make test-mutation` runs gomutants, which `tools/go.mod` pins, on the packages that `MUTATION_PACKAGES` lists in `make/go.mk`: the domain and the use cases of each core context, where the rules of the business live (TPL-0003).
For Reservation these rules are the overlap of periods, the capacity of a room, the grid of 15 minutes, the longest period, the past, and the notice before a cancellation.
The supporting contexts are not mutated. Their use cases are transaction scripts whose tests run against a database, so a run would be slow and would say little.
A mutant runs only the tests of its own package, so the tests of the domain must hold the rules of the domain themselves.
A mutant that compiles to the same machine code as the code is equivalent, and gomutants drops it by itself (`--detect-equivalent`).

The check runs when a Go file of a listed package, `backend/go.mod`, `equivalent-mutants.allow`, or the definition of the check is in scope.
In November 2026, a run over the domain and the use cases of Reservation made 338 mutants and took about 100 seconds on a laptop busy with other work, with 4 mutants at a time.
When you add a core context, add its `internal/domain` and `internal/app` to `MUTATION_PACKAGES`.

### Frontend: test-mutation-frontend

`make test-mutation-frontend` runs StrykerJS with its tap runner, which `frontend/package.json` pins, on each module of `frontend/src/` that has a unit test (`*.test.ts`) next to it: the pure logic that reads the forms, chooses the words for each kind of problem, and converts times in a time zone.
`frontend/stryker.config.mjs` finds the modules from their tests, so a new module with a test is mutated without an edit. Components and server modules have no unit test, because the browser tests them (`make e2e`), and they are not mutated.

- The unit tests run on the test runner of Node as `make test-frontend` runs them, each test file in a process of its own. StrykerJS counts a test file as one test, and runs a mutant against every test file that reaches its line, which can be the test of a module that imports the mutated one.
- StrykerJS stops a loop that never ends by counting how often the mutant runs, which does not depend on the machine, and anything else by time. The floor of that time is 30 seconds, so that a busy machine does not count a slow test as a hang and hide a mutant that lives: with a floor of 5 seconds, a machine with a load of 100 turned 52 mutants into timeouts.
- StrykerJS does not detect equivalent mutants.

The check runs when a mutated module or its test, `frontend/stryker.config.mjs`, `frontend/package.json`, `equivalent-mutants.allow`, `tools/stryker/`, or the definition of the check is in scope.
In November 2026, a run over the 8 modules with tests made 737 mutants and took about 75 seconds on a laptop, with 4 mutants at a time. Three runs gave every mutant the same status.

## Run part of it

```sh
make test-mutation MUTATION_PACKAGES=backend/internal/reservation/internal/domain
make test-mutation-frontend MUTATION_MODULES=frontend/src/lib/input.ts
```

The reports, with every mutant and its status, stay in `.cache/mutation/backend.json` and `.cache/mutation/frontend.json`.
A run of some modules of the frontend leaves out the allowances of the other modules.

## Read a finding

A mutant that lives gives a finding such as these:

```text
backend/internal/reservation/internal/domain/reservation.go:168: [mutation/survived] every test passes with `<` changed to `<=` (CONDITIONALS_BOUNDARY, mutant backend/internal/reservation/internal/domain/reservation.go:checkTerms:CONDITIONALS_BOUNDARY#2). ...
frontend/src/lib/input.ts:32: [mutation/survived] every test passes with `number >= min` changed to `number > min` (EqualityOperator, mutant frontend/src/lib/input.ts:wholeNumber:EqualityOperator#1). ...
```

The first line is `attendees < 1`, in the rule that a reservation has at least one attendee. No test booked a room for exactly one attendee, so a rule of "at least two" would pass every test.
The second is the lower limit of a whole number in a form. No test entered 1, the smallest number that the forms accept, so a form that refused it would pass every test.

A mutant of Go is named `<file>:<function>:<type>#<n>`: the n-th place of that type in that function.
A mutant of the frontend is named `<file>:<declaration>:<Mutator>#<n>`: the n-th mutant of that mutator in that top-level declaration, a function or a constant such as a table of words. StrykerJS numbers its mutants in sequence over a whole run, so `tools/stryker/mutants.jq` makes these names.
A change elsewhere in the file leaves a name as it is; a change in the same function or declaration can renumber it.
`[mutation/not-covered]` names a line that no test runs at all.

To run one mutant alone, with its verdict:

```sh
make mutant ID='backend/internal/reservation/internal/domain/reservation.go:checkTerms:CONDITIONALS_BOUNDARY#2'
make mutant-frontend ID='frontend/src/lib/input.ts:wholeNumber:EqualityOperator#1'
```

Each prints `KILLED` when a test kills the mutant, and `LIVED` (gomutants) or `SURVIVED` (StrykerJS) when it lives, and fails when it lives.
StrykerJS cannot run a mutant by its name, so `mutant-frontend` runs every mutant of the declaration and reads the one; it takes a few seconds.
To see the mutant fail or pass a test by hand, make the change of the finding at its line, run the tests (`go test` on the package, or `make test-frontend`), and undo the change.

## Fix a survivor

A mutant that lives asks one of two questions.

- Does a test check what this code does? Usually not, and the fix is a test that fails with the change. Write it in the style of the tests of the package or the module, often as a new row of a table test, and name the rule that it holds. For the findings above, the fixes are a booking for the organizer alone and a form with the number 1.
- Is the code needed? A condition that a later check repeats, or a branch that cannot happen, makes a mutant that no test can kill. Make the code simpler, in a commit of its own. For example, a use case once tested `found || err != nil` while every error came with `found` set; it now tests one flag. The readers of the forms of the frontend tested the period and the attendees again after they had written the messages, only for the type checker; they now write the messages in the branch that fails, so each condition decides something.

A test that only executes a line kills nothing: assert the result that the change would alter.
If the survivor shows that the code is wrong, not only untested, fix the code, and say so in the commit.

## Words that a screen shows

In the frontend, many mutants empty a string: a sentence that a screen shows, such as the message of a field or the words for a problem. Tests that pinned every sentence would make each change of wording a change of a test; tests that pinned none would let a message become empty unseen. The tests of this repository do neither.

- A sentence that carries a fact is pinned word for word: a number ("Enter a whole number from 1 to 1000."), a rule ("A reservation can be cancelled until 30 minutes before it starts."), or the field that it is about ("Enter the name of the room."). A change of these words changes what the screen tells, so it is a change of a test too.
- Every other sentence is held by a rule: a message is a sentence, with a capital letter first and a period last. The rule fails when a message is empty, whatever its words.

No string is left out of mutation.

## When an allowance is right

Some mutants are equivalent: no test can tell them from the code, because the change cannot alter what the code does.
Prefer to change the code until the mutant is gone. For example, the opening and the closing of a room were each built with their own seconds, and moving only one of them by one second changed nothing, because periods lie on the grid of 15 minutes. Both now come from one function, so the change moves both, and the tests of the bounds notice it.
In the frontend, a cache of the formatters of time zones made two mutants that only time could tell apart: a cache that always misses, and one that keeps nothing. Making a formatter takes about 40 microseconds, and a page reads the clocks a few dozen times, so the cache is gone.

When the code cannot change, `equivalent-mutants.allow` at the root lets the mutant live, one line per mutant:

```text
# <mutant> | <original> -> <replacement> | <reason>
```

- The mutant and the change are written as the finding names them.
- A line belongs to the check of the language of the file that its mutant names: a JavaScript or TypeScript file to `test-mutation-frontend`, any other file to `test-mutation`, which fails on a line that names neither a Go file nor a JavaScript or TypeScript one.
- The reason proves that the change is equivalent. "Hard to test" is not a proof: a mutant that a test could kill is a test to write.
- Adding a line is a decision of review. An agent stops and asks, with the finding and the proof, instead of writing the line.
- A line that matches no living mutant fails the check (`mutation/unused-allowance`), and so does a line without its reason (`mutation/allowance-format`). When a test kills the mutant, or the code changes, the line has to go, so the file does not rot.
