---
status: accepted
date: 2026-11-17
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

- The `database` module takes a variable that selects Serverless v2, with a minimum and a maximum capacity, or provisioned instances.
- stg uses Serverless v2 with a minimum of 0 ACU, so it pauses when idle.
- prd chooses by its needs, for example provisioned instances for a steady load.
- One open connection keeps the cluster awake. In stg, the connection pool therefore keeps a minimum of zero connections, and closes a connection that has been idle for a few minutes.
- The health check of a service does not query the database. A health check that connects every few seconds would never let the cluster pause.
- Resuming takes about 15 seconds, and 30 seconds or more after a pause longer than 24 hours. Connections use a timeout longer than that, and retry.
- RDS Proxy is not used. It keeps a connection open to each instance, so a cluster with RDS Proxy never pauses.
- Logical replication is not used either. The Aurora documentation on automatic pause, linked below, names it as another condition that keeps a cluster from pausing.

### Consequences

- Good, because an idle stg pays for storage only. Compute is charged only while the cluster runs.
- Good, because stg and prd use the same module and the same engine. Only the kind and amount of capacity differ.
- Good, because Serverless v2 also scales up within its range under load, without a change of instance class.
- Bad, because the first request after a pause waits about 15 seconds or longer, which people and tests in stg notice.
- Bad, because the code follows rules that exist only for pausing (a pool with no minimum of connections, a health check that leaves the database alone, retries on connect), and one mistake, such as a tool left connected, silently keeps stg awake and costing money.
- Bad, because stg and prd may use different kinds of capacity, so the behavior of stg under load does not predict prd exactly.
- Bad, because scheduled jobs inside the database, such as those of `pg_cron`, do not run while the cluster is paused, and autovacuum work in progress is canceled when it pauses.

### Confirmation

M0 has no infrastructure code, so nothing enforces this decision yet.
With the infrastructure milestone, variable validation in the `database` module rejects values that do not fit the selected kind of capacity, such as a minimum greater than the maximum.
Review checks the settings of the connection pool and of the health checks for stg.

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
- Not yet verified: whether Serverless v2 with a minimum of 0 ACU is available for Aurora PostgreSQL 18. AWS lists the engine versions that support 0 ACU, and for earlier major versions the support began at a particular minor version (for example 16.3). This is checked when the infrastructure is built.
- [Scaling to zero ACUs with automatic pause and resume](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless-v2-auto-pause.html) in the Aurora documentation
