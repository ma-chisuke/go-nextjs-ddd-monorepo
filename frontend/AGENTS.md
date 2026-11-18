# AGENTS.md (frontend)

Instructions for coding agents in `frontend/`, in addition to the root [AGENTS.md](../AGENTS.md). [ARCHITECTURE.md](../ARCHITECTURE.md) is the map.

## Read the documentation of the installed Next.js first

This app runs Next.js 16 with the App Router, React 19, and TypeScript 6 (`package.json`). Next.js 16 differs from what you may know, and the app uses Cache Components, partial prefetching, and typed routes (`next.config.ts`).
Before you write code, read the guide for it under `node_modules/next/dist/docs/01-app/` in this directory: the documentation of the installed version. `next dev` would write a block that says so into this file; `next.config.ts` turns that off (`agentRules: false`).

## Commands

| Command | Use |
|---|---|
| `make gen` | Regenerate `src/lib/api/<context>.gen.ts` from each `contracts/http/<context>.openapi.yaml` (openapi-typescript). |
| `make test-frontend` | Run the unit tests, `src/**/*.test.ts`, on the test runner of Node. |
| `make frontend-dev` | Run `next dev` at <http://127.0.0.1:3000> against the APIs of the local system that `make up` started. |
| `make up` | Start the local system; the frontend is at <http://127.0.0.1:13000>. |
| `make e2e` | Start the local system and run the scenarios of `e2e/` with Playwright in Chromium. Needs Docker. |

- `make check` does not run `next build`. The image builds it (`Dockerfile`), in `make up`, `make e2e`, and `make check-all`. The build fails when a page reads request data or uncached data outside `<Suspense>`.
- Procedures: [add a screen](../docs/guides/add-a-screen.md), [change an API](../docs/guides/api-changes.md), [run locally](../docs/guides/run-locally.md); skill `add-screen`.

## Where code goes in `src/`

| Code | Place |
|---|---|
| A page that composes features, and its `not-found.tsx` | `app/<route>/` |
| What one context gives, turned into what another needs | `app/_compose/` |
| The calls to the API of a context, through `outcome` | `features/<context>/server/*.ts`, which imports `server-only` |
| The Server Functions of the forms of a context | `features/<context>/actions.ts` (`"use server"`) |
| Pure logic: reading a form, the words for each kind of problem | `features/<context>/*.ts`, with `*.test.ts` next to it |
| The components of a context | `features/<context>/components/` |
| The API clients and `outcome`, problems, form input, times in a time zone, shared components | `lib/api/server/`, `lib/problem.ts`, `lib/input.ts`, `lib/zoned-time.ts`, `lib/components/` |

A module with unit tests imports nothing of Next.js, and imports other modules by relative paths with the extension `.ts`, because Node runs it as it is.

## Rules that the checks enforce

- `boundaries/dependencies`: a feature never imports another feature, and `lib` never imports a feature; a page composes them. (TPL-0003)
- `no-restricted-syntax`: a module under `server/` imports `server-only`, and a client component imports no module under `server/`. Only the server of Next.js calls the APIs. (TPL-0014)
- `gen-check-api-ts`: `src/lib/api/*.gen.ts` is what `make gen` writes. Do not edit it. (TPL-0006, TPL-0007)
- `lint-frontend-types`: a call that the API document does not allow, or a link to a route that does not exist, fails `tsc`. (TPL-0007)
- `lint-frontend-unused`: no file, export, or dependency that nothing uses.
- `test-frontend`, and `e2e` in `make check-all`: pure logic has unit tests; what a screen does is tested in a browser.

## Rules that review enforces

- A screen tells problems apart by `problemKind` and maps invalid parts to fields with `fieldErrors`; it never parses a title or a detail. (TPL-0008)
- A Server Function narrows each value of its `FormData` with `lib/input.ts`, and returns a state for an expected failure instead of throwing.
- A booking sends the Idempotency-Key of its form, and a change sends the version that the page showed in `If-Match`: read the comments at the top of `features/reservation/booking.ts` and `changes.ts`.
- Plain CSS in `app/globals.css`, no component library, no date library. Known limits are in the guide [add a screen](../docs/guides/add-a-screen.md#known-limits).
