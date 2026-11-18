# Add a screen

A screen of the frontend shows what the API of one bounded context gives, or composes several contexts on one page, and a form on it changes what the API holds.
The steps name the files of a context `<context>` under `frontend/src/`; [ARCHITECTURE.md](../../ARCHITECTURE.md#the-frontend) shows what each place holds.
The facility feature (`features/facility/`) is the small example: a list, a page, and a form that registers a room. The reservation feature is the large one: a form with an idempotency key, and forms that send a version.

Next.js 16 differs from what you may know. Before you write a part, read its guide under `frontend/node_modules/next/dist/docs/01-app/`.

## Steps

1. Start from the API. If the screen needs an operation that the API of the context lacks, add it first ([Add an endpoint](add-an-endpoint.md)). `make gen` writes the types of every API document to `src/lib/api/<context>.gen.ts`.
   For the first screen of a context, add its client to `src/lib/api/server/clients.ts`, in the form of `facilityApi`, with a variable for its base URL. Give the variable to the service `web` in `compose.yaml`, and to the target `frontend-dev` in `make/frontend.mk`.

   Check: `make check` runs `gen-check-api-ts`, which fails while the types are stale (`gen/stale`), missing (`gen/missing`), or have no document (`gen/no-document`).

2. Read the data on the server, in `src/features/<context>/server/<name>.ts`. The module imports `server-only`, calls the client of the context through `outcome` from `lib/api/server/outcome.ts`, and returns a `Result`: the value of the answer, or its problem. `outcome` turns an API that does not answer into a problem too, and waits for the request, so no data of the build reaches a static page. A function that a page and its `generateMetadata` both call is wrapped in `cache` of React, as `getRoom` is, so the API is asked once.

   Check: `make check` fails on a module under `server/` that does not import `server-only` (`lint-frontend`, `no-restricted-syntax`), and on a call that the API document does not allow, such as a parameter that it does not declare (`lint-frontend-types`, `TS2353`).

3. Write the pure logic in `src/features/<context>/<name>.ts`: what a form holds and how a Server Function reads it, the request in the type that the API document gives (`components["schemas"]`), and what the screen says about each kind of problem.
   - Narrow each value of the form with `lib/input.ts`, and convert a date and a time of day in a time zone with `lib/zoned-time.ts`.
   - Tell problems apart by `problemKind`, the name of the page of the problem in `docs/problems/`. Give the kinds that a person can act on words of their own, at the field that they are about, and map the invalid parts of any other problem to fields with `problemFieldErrors`. Never parse a title or a detail (TPL-0008).
   - Import nothing of Next.js, and import other modules by relative paths with the extension `.ts`, because the test runner of Node runs the file as it is.

   Test it in `<name>.test.ts` next to it, with `node:test` and `node:assert/strict`, as `registration.test.ts` does.

   Check: `make check` runs the tests (`test-frontend`), and fails on an export that nothing uses (`lint-frontend-unused`).

4. Write the Server Function in `src/features/<context>/actions.ts`, under `"use server"`. It takes the state that the form shows and the `FormData` of the submit, and returns the next state:
   - it reads the form with the logic of step 3, and returns the state with a message at each field that does not fit;
   - it calls the server module of step 2, and returns the state of the problem when the API refuses;
   - after a success, it goes to the next page with `redirect`, or reads the page again with `refresh` from `next/cache`.

   Anyone can post to a Server Function, so it trusts no value of the form, and it returns a state for an expected failure instead of throwing. It lives outside `server/`, because the form, a client component, imports it.

5. Write the components in `src/features/<context>/components/`.
   - A server component reads its data through step 2, and shows a problem with `ProblemNotice` from `lib/components/`.
   - A form is a client component (`"use client"`). It calls the Server Function through `useActionState`, renders each field with `Field`, shows a message about the whole form with `ProblemNotice`, and moves the focus after a failed submit with `useFocusOnProblem`. It sets `noValidate`, because the Server Function checks every field.
   - Hidden fields carry what the Server Function needs and the person does not enter, such as the id of the room, the version of a reservation, or an Idempotency-Key. A value that must differ for each request, such as a new Idempotency-Key, is made by a server component behind `connection()`, never at build time.

   Check: `make check` fails when a client component imports a module under `server/` (`lint-frontend`, `no-restricted-syntax`).

6. Compose the page in `src/app/<route>/page.tsx`. The page renders its static parts at once, and wraps each part that reads live data in `<Suspense>` with a fallback.
   A page that shows two contexts reads each through its own feature, and hands what one gives to the other through a function in `src/app/_compose/`, as `reservationRoom` turns a room of Facility into the `Room` of Reservation.
   For an id that does not exist, call `notFound()`, and write a `not-found.tsx` next to the page. Link to a page with `Link`.

   Check: `make check` fails when a feature imports another feature (`lint-frontend`, `boundaries/dependencies`), and on a link to a route that does not exist (`lint-frontend-types`). The build of the image fails when a page reads request data or uncached data outside `<Suspense>`; `make check` does not build it, and `make up` and `make e2e` do.

7. Add a scenario to `frontend/e2e/`: what a person does, from the first screen to what the change causes, with the helpers of `e2e/support.ts`. Each scenario makes its own room and organizer with `unique`, so the scenarios run side by side. Find a field with `field`, which takes the visible one: Next.js keeps the page that a person came from in the document, hidden.

   Check: `make e2e` runs the scenarios against the local system (Docker). A failed scenario leaves a trace and a screenshot in `.cache/playwright/results/`, and a report in `.cache/playwright/report/`.

8. Update the part on screens in the README of the context, `docs/contexts/<context>/README.md`, and its translation. Then run `make i18n-stamp`.

9. Finish:

   ```sh
   make fmt
   make check
   make e2e   # the scenarios in a browser (Docker)
   ```

## What no check reports

- A screen that reads the title or the detail of a problem to decide what to do.
- A Server Function that uses a value of the form before it narrows it.
- A form that sends a booking without the Idempotency-Key of the form, or a change without the version that the page showed. The comments at the top of `features/reservation/booking.ts` and `changes.ts` describe both.
- A screen that the README of its context does not name.

## Known limits

- In a production build, a submit without JavaScript that fails, on a page with live data, answers with the streamed part of the page alone. With JavaScript, and for a submit that succeeds, the page is whole. This was found on Next.js 16.4.6, and is not reported to Next.js yet.
- A page for a room or a reservation that does not exist answers with status 200 and the screen of `not-found.tsx`. The lookup runs inside `<Suspense>`, after the status has gone out.
- The list of time zones of the form that registers a room comes from `Intl` of the server, which may differ from the time zone data of Go that Facility checks against. A room whose time zone the server of Next.js does not know shows no reservations.
- `outcome` keeps the body of an answer and drops its headers, such as `Retry-After`, so a screen cannot say how long to wait.
- Rescheduling a reservation that has started answers `period-in-past`, the kind of a period in the past, so the screen names both causes in one message.
- The unit tests cover the modules of pure logic. The components and the Server Functions are tested in the browser only, by `make e2e`.
- The template has no design system and no sign-in. The CSS is plain (`app/globals.css`), with no component library, and the team that copies the template chooses both.
