# Change an event

The JSON Schema of an event, in `contracts/events/<context>/<Name>.v<N>.json`, is its contract with the contexts that consume it (TPL-0013).
A consumer is written against one version of the schema, and it goes on reading events of that version after the publisher changes.
Two checks in `make check` guard the contract: `lint-events` checks that each schema is well formed, and `lint-events-breaking` compares each schema with its version on the base branch and fails on a change that a consumer of that version could not read.

## Make a change

A change that every consumer of the version can still read goes into the file of that version:

- a new property, required or not; a consumer ignores what it does not know;
- a property that becomes required;
- a changed `description`, `title`, `examples`, `$comment`, or `deprecated`.

Any other change is a new version of the event:

1. Copy `<Name>.v<N>.json` to `<Name>.v<N+1>.json` next to it, change the copy, and keep the old file as it was. The type of the new version, `<context>.<Name>.v<N+1>`, names the new file.
2. Add a message for the new version to the AsyncAPI document, as [Add an event](#add-an-event) describes.
3. Publish both versions until every consumer reads the new one, and mark the old schema `"deprecated": true`, which is a compatible change.

`lint-events-breaking` fails when a schema file of the base is deleted or renamed, even when no consumer reads that version any more.

## What lint-events checks

`lint-events` checks every file in `contracts/events/` whose name ends in `.json`. A schema depends on the AsyncAPI document of its context, so a change to any event contract checks every schema, whatever `CHECK_SCOPE` or `CHECK_FILES` says.

- `events/file-name`: the path is `contracts/events/<context>/<Name>.v<N>.json`, with the context in lower case letters, the name in UpperCamelCase, and the version a positive integer without a leading zero.
- `events/json`: the file is one JSON value, and no key appears twice in an object.
- `events/dialect`: `$schema` is `https://json-schema.org/draft/2020-12/schema`.
- `events/compile`: the schema compiles under JSON Schema 2020-12. A `$ref` may name a file next to the schema; nothing is read from the network.
- `events/title`: `title` is `<Name>`.
- `events/description`: the schema has a `description`, and so does every property at every depth, as a string that is not blank.
- `events/type`: the schema is a JSON object, and its `type` is `"object"`.
- `events/closed`: no schema in the file sets `additionalProperties` or `unevaluatedProperties` to `false`. A consumer ignores the properties that it does not know, and a closed object would make every new property a breaking change.
- `events/required`: in every schema in the file, each name in `required` is a key of `properties` of the same schema.
- `events/examples`: `examples` has at least one entry, and each entry matches the schema, formats included.
- `events/asyncapi`: the AsyncAPI document of the context, `contracts/events/<context>.asyncapi.yaml`, has a message whose `name` is `<context>.<Name>.v<N>` and whose `payload` refers to the file with a `$ref`, directly or through a reference inside the document (`#/...`). A message is any mapping in the document that has a `name` and a `payload`.

"At every depth" and "in the file" mean every schema inside the schema, which these keywords hold:

- the members of `properties`, `patternProperties`, `dependentSchemas`, and `$defs`;
- the entries of `allOf`, `anyOf`, `oneOf`, and `prefixItems`;
- the values of `items`, `additionalProperties`, `unevaluatedProperties`, `unevaluatedItems`, `contains`, and `propertyNames`, which apply to arrays and objects;
- the values of `not`, `if`, `then`, `else`, and `contentSchema`.

## What lint-events-breaking checks

`lint-events-breaking` compares each schema in scope with the same file at the merge base of `origin/main`, or of `CHECK_BASE` when it is set, as CI does.
A schema that does not exist at the base is new, and passes. Without a base, nothing is compared.

The two versions are compared from the root, keyword by keyword, and every schema inside them by the same rules:

- `description`, `title`, `examples`, `$comment`, and `deprecated` may change.
- `events/property-removed`: every key of `properties` at the base is still there. A new key passes.
- `events/required-removed`: every name in `required` at the base is still there. A new name passes.
- `events/keyword-changed`: every other keyword has the same value as at the base, and none is added or removed, even when the change only narrows what a value may be. Keywords that hold schemas are compared schema by schema instead of as a whole:
  - `items`, `additionalProperties`, `unevaluatedProperties`, `unevaluatedItems`, `contains`, and `propertyNames`: the schema in both versions.
  - `not`, `if`, `then`, `else`, and `contentSchema`: the schema in both versions.
  - `allOf`, `anyOf`, `oneOf`, and `prefixItems`: as many schemas in both versions, in the same order.
  - `patternProperties` and `dependentSchemas`: the same keys in both versions. `$defs` may gain a key.
- `events/schema-deleted`: every schema file of the base still exists at its path. A deleted file is never in scope, so a change that only deletes a schema fails in a run over every file, such as `make check` without a scope, which CI runs.

Each finding names the JSON pointer of the place, what it was, and what it is. The fix is a new version: publish the change as `<Name>.v<N+1>.json`, and keep the file of the old version as it was at the base.

## Add an event

1. Write the schema in `contracts/events/<context>/<Name>.v1.json`, in the form of `contracts/events/reservation/ReservationConfirmed.v1.json`, and leave the object open. It has:

   - `$schema`, `title`, `"type": "object"`, and `required`;
   - a `description` that says what happened and when the context publishes it;
   - a `description` for each property;
   - an example of real data in `examples`.

   Check: `lint-events`.

2. Add the event to the AsyncAPI document of the context, `contracts/events/<context>.asyncapi.yaml`, or write the document in the form of `contracts/events/reservation.asyncapi.yaml`:

   - a message named `<context>.<Name>.v1` whose payload is the CloudEvents envelope with `data: {$ref: ./<context>/<Name>.v1.json}`;
   - the message in the channel of the context;
   - an operation that sends it.

   Check: `lint-contracts` lints the document, and `lint-events` fails until the message refers to the schema.

3. In the tests of the publisher, pass each event that it produces, built with `event.New` and the type `event.Type("<context>", "<Name>", 1)`, through `eventtest.AssertMatchesContract` (`backend/internal/platform/event/eventtest`). It fails the test when the data does not match the schema of its type, formats included.

   Check: `test-go`.
