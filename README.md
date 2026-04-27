# pulse

A high-throughput event ingestion service in Go on PostgreSQL 17. The point of this project is not the API surface — it is the database underneath: time-partitioned storage, streaming + logical replication, pgBouncer + Patroni HA, WAL-G PITR, and full Prometheus/Grafana observability.

Every meaningful design decision is recorded in [`docs/architecture.md`](docs/architecture.md). Every meaningful query optimization is recorded in [`docs/perf-log.md`](docs/perf-log.md) with `EXPLAIN (ANALYZE, BUFFERS)` before/after.

## Stack

- **Go 1.23** — services (`ingest-api`, `aggregator`, `query-api`)
- **PostgreSQL 17** — primary store, declarative range partitioning, streaming replication
- **pgBouncer** — transaction pooling (week 6)
- **Patroni + etcd** — HA coordination (week 6)
- **Prometheus + Grafana + postgres_exporter** — metrics, dashboards, alerts (week 8)
- **WAL-G + MinIO** — base backups and PITR (week 7)
- **Python 3.12** — load generation, restore drills, ETL helpers
- **goose** — migrations
- **testcontainers-go** — integration tests
- **GitHub Actions** — CI

## Quick start

```bash
cp .env.example .env
make up           # start PostgreSQL primary
make migrate-up   # apply migrations
make run          # start ingest-api on :8080
```

Send a sample event:

```bash
curl -X POST http://localhost:8080/events \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"u-1","event_type":"page_view","properties":{"path":"/pricing"}}'
```

Expect `202 Accepted`. The row lands in the partition matching today's date.

## Layout

```
cmd/                Service entrypoints
  ingest-api/       HTTP write API (week 1)
  aggregator/       Background rollups, SKIP LOCKED queue (week 5)
  query-api/        Read API on replica (week 4)
internal/           Private application code
deploy/             Postgres / pgBouncer / Prometheus / Grafana configs
migrations/         goose SQL migrations
tools/              Python utilities (loadgen, restore drill, seed)
docs/               Engineering log: architecture, performance, runbooks
```

## Roadmap

See [`docs/roadmap.md`](docs/roadmap.md). Nine weekly milestones from "first POST returns 202" through HA failover, PITR, and observability.

## Engineering log

The interesting reading lives in [`docs/perf-log.md`](docs/perf-log.md): slow query case studies, EXPLAIN plans, indexing decisions, measured impact.
