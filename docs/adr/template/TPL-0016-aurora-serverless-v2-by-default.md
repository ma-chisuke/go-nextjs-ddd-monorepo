---
status: accepted
date: 2026-11-19
---

# TPL-0016: Use Aurora Serverless v2 by default, and choose the capacity for each environment

## Context and Problem Statement

All contexts share one PostgreSQL 18 database (TPL-0009).
On AWS, it can run on Amazon Aurora PostgreSQL or on Amazon RDS for PostgreSQL.
Aurora offers two kinds of capacity: provisioned instances of a fixed size, and Serverless v2, which scales between a minimum and a maximum number of Aurora capacity units (ACU).

The environments need different things.
stg is idle most of the time (nights, weekends, the hours between test runs), and should cost as little as possible while idle.
prd needs predictable performance, and its load may be steady enough that a fixed instance costs less.

Serverless v2 can scale down to 0 ACU and pause when no connection is open, so an idle stg pays only for storage.
Pausing has conditions and costs of its own: one open connection keeps the database awake, and the first connection after a pause waits while the database resumes.

Which kind of capacity does each environment use, and what must the code do to work with a database that pauses?

## Decision Drivers

- stg costs almost nothing while idle.
- prd chooses its capacity by its load, without changes to the modules.
- One module serves both environments, and the difference is in values (TPL-0015).
- The services keep working when the database pauses: connections wait and retry instead of failing.

## Considered Options

- Aurora with a variable that selects Serverless v2 or provisioned instances, with Serverless v2 as the default
- Provisioned Aurora instances in every environment
- RDS for PostgreSQL
- Serverless v2 in every environment, with a minimum greater than zero

## Decision Outcome

Chosen option: "Aurora with a variable that selects Serverless v2 or provisioned instances, with Serverless v2 as the default", because it lets stg pause at almost no cost, while prd keeps the choice of provisioned instances, with one module.

The rules are as follows.

- The `database` module takes a variable, `capacity`, that selects exactly one kind: Serverless v2, with a minimum and a maximum capacity and the time until it pauses, or provisioned instances of one class. `instance_count` adds readers to the writer, and one of them takes over when the writer fails.
- stg uses Serverless v2 from 0 to 2 ACU with one instance, so it pauses when idle.
- prd chooses by its needs. The template gives it two provisioned instances of the class `db.r8g.large`, a writer and a reader, for a steady load.
- One open connection keeps the cluster awake. The connection pool of every service therefore keeps a minimum of zero connections, and closes a connection that has been idle for three minutes (`platform/postgres`).
- The health check of a service does not query the database. A health check that connects every few seconds would never let the cluster pause.
- Resuming takes about 15 seconds, and 30 seconds or more after a pause longer than 24 hours. Connections use a timeout longer than that, and retry.
- RDS Proxy is not used. It keeps a connection open to each instance, so a cluster with RDS Proxy never pauses.
- Logical replication is not used either. The Aurora documentation on automatic pause, linked below, names it as another condition that keeps a cluster from pausing.

The module `infra/modules/database` builds the cluster under these rules as well.

- The storage, the backups, and the snapshots are encrypted with a customer managed key of the module, which rotates. The cluster is protected from deletion where the environment says so, and a destroy leaves a final snapshot.
- RDS makes the password of the master user, keeps it in a secret of Secrets Manager, and rotates it. Only the task that prepares the database signs in as the master user.
- Each context gets two secrets, one for its owner role and one for its runtime role (TPL-0009), each a JSON object with the keys `username`, `password`, and `url`. The password is an ephemeral value, written through a write-only argument, so it never enters the state of Terraform.
- A service gets `DATABASE_URL` from the key `url` of the secret of its runtime role, and connects with TLS (`sslmode=require`).
- Before a deployment, one ECS task runs the image of `db/Dockerfile`, whose script `db/prepare.sh` waits for a cluster that may be resuming, runs `db/bootstrap.sql` as the master user, sets the passwords of the roles from their secrets, and applies the `schema.sql` of every context with pgschema (TPL-0009, TPL-0010). It tries the first connection five times, and each connection waits 30 seconds.

### Consequences

- Good, because an idle stg pays for storage only. Compute is charged only while the cluster runs.
- Good, because stg and prd use the same module and the same engine. Only the kind and amount of capacity differ.
- Good, because Serverless v2 also scales up within its range under load, without a change of instance class.
- Bad, because the first request after a pause waits about 15 seconds or longer, which people and tests in stg notice.
- Bad, because the code follows rules that exist only for pausing (a pool with no minimum of connections, a health check that leaves the database alone, retries on connect), and one mistake, such as a tool left connected, silently keeps stg awake and costing money.
- Bad, because stg and prd may use different kinds of capacity, so the behavior of stg under load does not predict prd exactly.
- Bad, because scheduled jobs inside the database, such as those of `pg_cron`, do not run while the cluster is paused, and autovacuum work in progress is canceled when it pauses.
- Bad, because the master user of Aurora is not a superuser, so it hands a schema to an owner role only as a member of that role, and inherits the rights of every owner role (TPL-0009).
- Bad, because `sslmode=require` encrypts the connection but does not verify the certificate of the server against the CA of RDS.
- Bad, because a new password of a role reaches a service only when the service starts again. Between the run of the task that sets the passwords and the start of the services, the new connections of a service fail.
- Bad, because nothing has been applied to AWS. AWS Emulator knows Aurora PostgreSQL up to version 17.9 and runs no database, so the rehearsal applies the module with 17.9, and `make test-db-image` runs the task against the local PostgreSQL instead.

### Confirmation

- The validation of `capacity` in the module `database` rejects both kinds of capacity or neither, a minimum greater than the maximum, a capacity off the steps of 0.5 ACU, a time to pause outside 300 to 86400 seconds, and the class `db.serverless` as a provisioned class. The tests of the module (`validate-terraform`, in `make check-all`) try each of these, check that the default is Serverless v2 from 0 ACU, and check that the cluster is encrypted and protected from deletion.
- `rehearse-infra` applies the module with the values of stg to AWS Emulator, and plans it with the values of prd (TPL-0017).
- `test-db-image`, in `make check-all`, runs the image that prepares the database against the local PostgreSQL as a master user that is not a superuser, twice, and checks what it prepared (TPL-0009).
- Review checks the settings of the connection pool and of the health checks.

## Pros and Cons of the Options

### Aurora with a variable that selects Serverless v2 or provisioned instances, with Serverless v2 as the default

- Good, because stg costs almost nothing while idle, prd is free to choose, and one module serves both.
- Bad, because of the delay to resume, and of the rules that the code must follow so that the cluster can pause.

### Provisioned Aurora instances in every environment

- Good, because performance is steady, the cost is predictable, and at a steady load a provisioned instance costs less than the same capacity in Serverless v2.
- Bad, because stg pays for an instance around the clock, or must be stopped and started by hand, and AWS starts a stopped cluster again by itself after seven days.

### RDS for PostgreSQL

- Good, because a small instance costs less than Aurora, and new major versions of PostgreSQL have usually reached RDS before Aurora.
- Bad, because it cannot scale to zero: an idle stg pays for its instance unless someone stops it.
- Bad, because in the common Multi-AZ instance deployment, failover takes longer than on Aurora, whose instances share one storage volume.

### Serverless v2 in every environment, with a minimum greater than zero

- Good, because there is no delay to resume and no rule for pausing, and one kind of capacity everywhere.
- Bad, because the minimum, at least 0.5 ACU, is charged around the clock in stg.
- Bad, because at a steady load in prd, it costs more than provisioned instances of the same capacity.

## More Information

- Aurora PostgreSQL has supported major version 18 since June 2026, with version 18.3 ([announcement](https://aws.amazon.com/about-aws/whats-new/2026/06/amazon-aurora-postgresql-major-version-18)).
- Verified in the documentation of AWS on November 19, 2026: Aurora PostgreSQL 18 exists as 18.3, released on June 11, 2026, 18.4, and 18.6. Serverless v2 with a minimum of 0 ACU is supported from 18.3, in the Asia Pacific (Tokyo) Region too. The time until a pause is 300 to 86400 seconds. Resuming takes about 15 seconds, and 30 seconds or more after a pause of over 24 hours. The module defaults to 18.6.
- Not yet verified on AWS, because nothing has been applied: the engine version, the pause at 0 ACU, how long the task that prepares the database waits while the cluster resumes, and what the master user of Aurora PostgreSQL 18 may do ([Deploy to AWS](../../guides/deploy.md#what-the-first-apply-must-confirm), and the README of the module).
- [Scaling to zero ACUs with automatic pause and resume](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless-v2-auto-pause.html) in the Aurora documentation
