# Change an API

The OpenAPI document of a context, in `contracts/http/`, is its contract with the clients that call it (TPL-0006).
The check `lint-contracts-breaking` compares each document with its version on the base branch, and fails on a change that breaks those clients (TPL-0007, layer 6).

## Make a breaking change on purpose

Removing a response property, adding a required request property, and the other changes that oasdiff rates as errors fail the check, with the rule of oasdiff and the line in the document.
When the change is intended, raise `info.version` of the document in the same pull request: below 1.0, raise the minor version (`0.1.0` to `0.2.0`); from 1.0 on, raise the major version (`1.4.0` to `2.0.0`).
oasdiff then accepts the breaking changes of that document, and the raised version shows the reviewer that the pull request breaks the contract on purpose.
The reviewer decides whether every client can follow, and the clients in this repository, such as the frontend and the generated client of another context, change in the same pull request.
The check compares with the merge base of `origin/main`, or of `CHECK_BASE` when it is set, as CI does. A new document has nothing to compare with, and passes.

## Follow the change in the clients

After a change to an API document, `make gen` regenerates the code on both sides of the contract: the Go server of the context and the clients of other contexts (oapi-codegen), and the TypeScript types of the frontend, `frontend/src/lib/api/<context>.gen.ts` (openapi-typescript).
The type check of the frontend then names the code of the screens that no longer fits the types: `lint-frontend-types` fails at each call whose path, parameters, or body the document no longer allows, and at each use of a part of an answer that is gone.
A breaking change updates those screens in the same pull request. `make e2e` shows whether the screens still work with the changed API.
