# Roadmap

Nine weekly milestones from "first POST returns 202" through HA failover, PITR, and observability. Each milestone has a single demo deliverable and a written log entry. The point is not to ship every feature on a checklist — it is to be able to walk a recruiter through any one of these and answer follow-up questions.

## Week 1 — Skeleton

Goal: `POST /events` returns 202 and the row lands in the right partition.

- Go HTTP service on chi, pgx pool, structured logging, graceful shutdown
- docker-compose with PostgreSQL 17 + tuned `postgresql.conf`
- goose migrations: parent `events` table partitioned by day, 14 daily partitions, default partition
- Unit tests on the handler, Makefile targets, baseline CI

Demo: `curl POST /events` → `psql -c "SELECT count(*) FROM events;"` returns 1.

## Week 2 — Schema & partition maintenance

Goal: partitions roll forward automatically.

- `internal/partitions` package: a Go routine that creates the next N partitions on a tick
- Migration 00003: `outbox` table, `event_aggregates` table (still empty)
- Decide on retention horizon and document it
- Idempotency on inserts (UNIQUE on `(user_id, event_type, properties->>'idempotency_key', created_at)` partial index)

Demo: stop the system, advance the system clock, restart — new partitions appear without human action.

## Week 3 — Indexes & EXPLAIN

Goal: four canonical query patterns run fast and the proof is in `docs/perf-log.md`.

- Pattern A: `SELECT * FROM events WHERE created_at >= $1 AND created_at < $2 AND user_id = $3` (btree on `user_id, created_at`)
- Pattern B: `... AND properties @> '{"path":"/x"}'` (GIN on `properties`)
- Pattern C: full-day scan for analytics (BRIN on `created_at`)
- Pattern D: `WHERE event_type = $1 AND created_at >= now() - '1 hour'` (partial index)
- For each: baseline EXPLAIN ANALYZE, the change, the new EXPLAIN ANALYZE, and a one-paragraph commentary

## Week 4 — Read replica & routing

Goal: writes go to primary, reads go to replica.

- Add `postgres-replica` to docker-compose with streaming replication
- Two pgx pools in the app, exposed via a `db.Cluster` interface
- New `query-api` service for analytical queries, hitting the replica
- Test: kill the primary, verify replica becomes read-only correctly

## Week 5 — Aggregator worker & SKIP LOCKED

Goal: async hourly rollups via a transactional outbox.

- `cmd/aggregator` claims jobs from `outbox` with `FOR UPDATE SKIP LOCKED`
- LISTEN/NOTIFY wakes the worker without polling
- Aggregates land in `event_aggregates` (count by user_id × hour × event_type)
- Idempotent, restart-safe, tested under concurrency with multiple workers

## Week 6 — pgBouncer + Patroni HA

Goal: unplug the primary node, the cluster keeps writing within seconds.

- pgBouncer in transaction pooling mode (note: prepared statements need `statement_cache_capacity=0` in pgx)
- Patroni + etcd cluster of three nodes
- Document failover behavior: what the app sees, what it retries
- Update the `db.Cluster` to reconnect through pgBouncer

## Week 7 — Backups & PITR

Goal: restore the database to an arbitrary point in time, in CI.

- WAL-G with MinIO as S3 backend
- Hourly base backup, continuous WAL shipping
- `tools/restore_drill.py`: nukes a fresh DB, restores to T-30 minutes, asserts data integrity
- CI job runs the drill weekly

## Week 8 — Observability

Goal: a single Grafana page tells you whether the system is healthy.

- postgres_exporter + Prometheus + Grafana, all in docker-compose
- Pre-built dashboards: TPS, replication lag, top queries from `pg_stat_statements`, bloat, autovacuum activity
- Application metrics from the Go services (request rate, latency histograms, queue depth)
- Alert rules: replication lag > 30s, default partition non-empty, outbox lag > 60s

## Week 9 — Polish & write-up

Goal: the README and `docs/perf-log.md` read like the work of someone who has done this before.

- Run loadgen at a sustained target (start at 1k RPS, push higher)
- Record before/after numbers in the perf log
- Architecture diagram (mermaid), failure-mode runbook, "what I'd do next" section
- Three-minute screen recording demo for the recruiter
