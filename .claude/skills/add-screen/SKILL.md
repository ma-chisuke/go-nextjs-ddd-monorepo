---
name: add-screen
description: Add a screen or a form to the Next.js frontend, from the server module that calls the API of a bounded context, through the pure logic with its unit tests, the Server Function, and the components, to the page that composes features and a scenario in a browser. Use when a change shows data of a context on a page, or adds a form that changes what an API holds.
---

# Add a screen

Follow `docs/guides/add-a-screen.md`. It has the steps, the files, and the check that confirms each step; this skill adds only what to verify.

## Steps

1. Read `frontend/AGENTS.md`, and the README of each context that the screen shows in `docs/contexts/<context>/`: its operations and the problems that each can answer.
2. Next.js 16 differs from what you know. Before you write a part, read its guide under `frontend/node_modules/next/dist/docs/01-app/`, such as Server Functions, Cache Components, or `<Suspense>`.
3. If the screen needs an operation that the API lacks, add it first with the skill `add-endpoint`, and run `make gen`. Never edit `src/lib/api/*.gen.ts`.
4. Follow the guide step by step: the server module, the pure logic and its unit test, the Server Function, the components, the page, and, for a new page or a new form, the scenario in `frontend/e2e/`. A change to what a page that exists shows needs the unit test of its logic, and no scenario of its own.
5. Verify, before you finish:
   - every call to an API goes through `outcome` in a module under `server/`, and every part of a page that reads data of a context sits inside `<Suspense>`;
   - the Server Function narrows each value of its `FormData` before it uses one, and returns a state for an expected failure instead of throwing;
   - the screen tells problems apart by `problemKind`, never by a title or a detail, and each problem that the operation can answer shows either words of its own or its title and detail;
   - a form that books sends the Idempotency-Key of the form, and a form that changes a reservation sends the version that the page showed;
   - a page that shows two contexts hands data from one feature to the other through `src/app/_compose/`, and no feature imports another;
   - the part on screens in the README of each context, and its translation, name the page.
6. Run `make fmt`, then `make check`. Run `make e2e` too when Docker is available; it builds the image, which fails on data read outside `<Suspense>`, and runs the scenarios.
