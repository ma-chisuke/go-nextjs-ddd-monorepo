---
status: accepted
date: 2026-11-17
---

# TPL-0009: Use one PostgreSQL 18 database, with a schema and roles for each context

## Context and Problem Statement

The backend is split into bounded contexts (TPL-0003), and contexts work together only through APIs and events (TPL-0005).
Each context owns its data. Another context learns about that data only from the owner's API or from its events.

That rule needs support in the database.
If the tables of every context sit side by side and every connection can read them all, a join across two contexts is one line of SQL, and nothing stops it.
At the other extreme, a database server for each context multiplies the cost and the operations, while the sample has three small contexts and most teams start with a few.

The engine also has to be chosen.
The template needs transactions, constraints over several rows (the sample rejects two reservations of one room whose periods overlap, with an exclusion constraint), and a native UUID type with UUIDv7 values (TPL-0012).

How do we store the data of all contexts so that no context can read or change the data of another, without running a database server for each context?

## Decision Drivers

- The database itself must stop a context from reading or changing the data of another context. Review alone is not enough.
- A context that outgrows the shared database must be able to move out without untangling queries and keys first.
- The cost and the operations must suit a small team in the local environment, in stg, and in prd.
- The engine must support exclusion constraints and generate UUIDv7 values (TPL-0012).

## Considered Options

- One PostgreSQL 18 database, with a schema and roles for each context
- A database cluster for each context
- One database, with every table in one shared schema
- One cluster, with a database for each context

## Decision Outcome

Chosen option: "One PostgreSQL 18 database, with a schema and roles for each context", because privileges in the database enforce the boundary between contexts while the team runs and pays for one database server.

The rules are as follows.

- One PostgreSQL 18 database serves every context.
- Each context owns one schema and two roles:
  - an owner role, which owns the schema and is the only role that changes it (TPL-0010);
  - a runtime role, which the context's services connect as. It can read and write the rows in its own schema, cannot change the schema, and has no privileges in any other schema.
- No foreign key, view, or query crosses from one context's schema into another's. Data that another context needs travels through APIs and events (TPL-0005).
- No transaction spans two contexts. A change that another context must learn about is published as an event (TPL-0013).
- `db/bootstrap.sql` creates what the schema tool does not manage: the schemas, the roles, and the extensions. One extension is `btree_gist`, which the exclusion constraint of the sample needs.

### Consequences

- Good, because the database enforces the boundary: a query by one context's runtime role on another context's table fails with "permission denied", whatever the code does.
- Good, because a context can later move to a database of its own. Nothing in its schema refers to another schema, so the move copies one schema and changes one connection setting.
- Good, because there is one server to run, back up, upgrade, and pay for in each environment.
- Bad, because the contexts share one failure domain and one set of resources: an outage, a heavy query, or a burst of connections in one context affects all of them.
- Bad, because the connection pools of all contexts draw on the connection limit of one server, so their sum must stay under that limit.
- Bad, because every context moves to a new major version of PostgreSQL at the same time.
- Bad, because no transaction spans two contexts. This is intended, but an event (TPL-0013) takes more code than one transaction, and the other context sees the change later.

### Confirmation

Nothing enforces this decision yet: M0 has no schema, no query, and no role.
The following checks arrive with the first bounded context.

- sqlc is configured per context and reads only that context's `schema.sql`, so a query on another context's table fails at generation (TPL-0011).
- The privileges of the roles make such a query fail at run time.
- A check of each `schema.sql` fails on a reference to another schema, such as a foreign key.
- An integration test connects as the runtime role of one context and asserts that reading the schema of another context fails.

Until then, review enforces the decision.

## Pros and Cons of the Options

### One PostgreSQL 18 database, with a schema and roles for each context

- Good, because privileges enforce the boundary, and one server serves every context.
- Good, because PostgreSQL supports exclusion constraints, and PostgreSQL 18 generates UUIDv7 values with the built-in function `uuidv7()`.
- Bad, because the contexts share the resources, the failures, and the version of one server.

### A database cluster for each context

- Good, because the isolation is complete: resources, failures, versions, and upgrades are separate.
- Good, because each context can size and tune its database on its own.
- Bad, because the cost and the work grow with the number of contexts: each cluster needs its own backups, monitoring, upgrades, and credentials, in every environment.
- Bad, because the local environment runs a PostgreSQL server for each context.

### One database, with every table in one shared schema

- Good, because it is the simplest setup, and most tools and examples assume it.
- Bad, because only names and review keep the contexts apart. Any connection can read and join every table, so the boundary wears away one query at a time.
- Bad, because moving a context out later starts with finding every query and foreign key that crosses its boundary.

### One cluster, with a database for each context

- Good, because PostgreSQL cannot run one query across two databases, so a context cannot reach into another at all.
- Good, because one server still serves every context.
- Bad, because the gain over schemas is small: the contexts still share the resources, the failures, and the version of the server.
- Bad, because work that looks across contexts, such as a read-only role for investigating an incident, needs a connection to each database.

## More Information

- [Schemas](https://www.postgresql.org/docs/18/ddl-schemas.html) and [privileges](https://www.postgresql.org/docs/18/ddl-priv.html) in the PostgreSQL 18 documentation
- The exclusion constraint of the sample is `EXCLUDE USING gist (room_id WITH =, period WITH &&)`. A GiST index supports `=` on a column such as `room_id` only with the extension `btree_gist`.
- The schema tool is pgschema (TPL-0010), the query generator is sqlc (TPL-0011), and the database on AWS is Aurora PostgreSQL (TPL-0016).
