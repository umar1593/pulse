# Architecture decisions

This is an append-only log. New decision = new section at the bottom. We do not edit old entries; if a decision is reversed we add a new entry that supersedes it and link back.

The format is a stripped-down [ADR](https://adr.github.io/): **Context** (why we are deciding), **Decision** (what we picked), **Alternatives** (what we did not pick and why), **Consequences** (what gets harder).

---

## ADR-001 — PostgreSQL for the event store

**Context.** We need a high-throughput event ingestion store. Candidates: PostgreSQL with partitioning, ClickHouse, Kafka + a warehouse downstream.

**Decision.** PostgreSQL 17 with declarative range partitioning on `created_at`.

**Alternatives.**

- *ClickHouse.* Better at columnar analytics, but deliberately not the focus of this project — the goal is to demonstrate PostgreSQL depth.
- *Kafka.* Excellent ingestion buffer, but introduces a second store to operate. We want to show that PostgreSQL alone can handle the workload at the target scale (1k–10k RPS sustained on a single primary).

**Consequences.** Aggregations require explicit rollups (week 5) instead of falling out of a columnar engine. Partition maintenance becomes our problem (week 2).

---

## ADR-002 — Daily partitions, not monthly or hourly

**Context.** Range-partitioning by time. Granularity tradeoff: too coarse means giant partitions and slow drops; too fine means thousands of partitions and planner overhead.

**Decision.** Daily partitions.

**Alternatives.**

- *Hourly.* 8760 partitions/year. Planner overhead becomes measurable on planning-time-sensitive queries.
- *Monthly.* Bigger drop windows for retention, but each partition is much larger; vacuum and index rebuilds become heavier.

**Consequences.** ~365 partitions per year per table; well within Postgres' comfort zone (we'll measure planning time in week 3). Retention policy operates on day boundaries.

---

## ADR-003 — Composite primary key `(id, created_at)` over surrogate `id`

**Context.** Partitioned tables require the partition key to be present in any unique constraint, including the primary key.

**Decision.** PK is `(id, created_at)` where `id uuid DEFAULT gen_random_uuid()`.

**Alternatives.** None — Postgres rejects PK on `id` alone for partitioned tables.

**Consequences.** Foreign keys from other tables would need to reference the composite. We don't have any FKs into `events` and don't plan to.

---

## ADR-004 — pgx v5, not database/sql + lib/pq

**Context.** Driver choice for the Go services.

**Decision.** `github.com/jackc/pgx/v5` with `pgxpool`.

**Alternatives.**

- *database/sql + lib/pq.* Standard but lacks first-class support for things we want — `COPY FROM`, `LISTEN/NOTIFY`, batching, native UUID/JSONB types.
- *sqlc on top of pgx.* Considered for type-safe query generation; deferred to week 5 when query count grows.

**Consequences.** We are mildly coupled to pgx types in the repository layer. Acceptable for a database-centric project.

---

## ADR-005 — Partition worker as its own process

**Context.** Daily partitions must keep rolling forward even if ingest traffic is low, and the maintenance path issues DDL that has very different failure modes from the hot write API. We want the ingest path to stay small, predictable, and easy to scale independently from operational jobs.

**Decision.** Run partition maintenance as a separate `partition-worker` process on an hourly ticker. This slightly increases operational surface area, but it gives us a smaller blast radius, clearer logs, and the option to supervise or restart the worker independently if a maintenance bug appears.

**Alternatives.**

- *Run it inside `ingest-api`.* Fewer moving pieces, but every API replica would need leader election or coordination to avoid duplicated work, and a bad maintenance loop would share fate with the write path.

**Consequences.** In a tiny deployment we could still fold this back into `ingest-api` if process count mattered more than isolation. What would change our mind: if the operational cost of another service outweighed the risk reduction, or if we later adopt an external scheduler that can own DDL jobs centrally.

---

## ADR-006 — Keep a 90-day hot retention horizon

**Context.** Partitioning only pays off operationally if we decide how long hot event data stays in the primary store. Without a declared horizon, partition drops are guesswork and disk growth becomes open-ended.

**Decision.** Treat 90 days as the hot-data retention horizon for `events`. The worker already has drop support, but we keep destructive drops disabled until the week 5 pipeline is in place; for now this is the documented policy the system is being built toward.

**Alternatives.**

- *30 days.* Cheaper on disk, but too short for the kind of recruiter demos and ad hoc performance investigations this project is meant to support.
- *Infinite retention on the primary.* Simplest short term, but it defeats the point of practicing partition lifecycle management.

**Consequences.** Daily partitioning keeps this manageable at roughly 90 active partitions for hot data. When the retention path is turned on, dropping old partitions becomes a fast metadata operation instead of a row-by-row delete.
