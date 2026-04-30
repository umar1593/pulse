# Week 2 — partition maintenance & schema additions

Goal at end of week: a separate Go process (`partition-worker`) runs on a ticker, automatically creates upcoming daily partitions, and the `outbox` and `event_aggregates` tables exist (still empty — they get used in week 5). You will write your first piece of original Go code as part of this week.

## What's already in the skeleton

I added these files. Pull them into your repo and read each one before running anything. Reading carefully is the actual learning here, not running the code.

- `migrations/00003_outbox_aggregates.sql` — adds `outbox` and `event_aggregates`.
- `internal/partitions/plan.go` — pure Go: given the current time and a list of existing partitions, returns the CREATE/DROP plan. No database.
- `internal/partitions/plan_test.go` — four unit tests on the planner.
- `internal/partitions/maintainer.go` — executes the plan against PostgreSQL using `pg_inherits` to enumerate partitions and Postgres `format()` to safely build DDL.
- `cmd/partition-worker/main.go` — entrypoint: loads config, opens pool, runs `Maintainer.Run` on a 1-hour ticker.
- `Makefile`: new target `run-partition-worker`.

## Steps in order

### 1. Pull the new files

Copy all five files above into your repo, in the same paths.

### 2. Read `plan.go` until you understand every line

Specifically: the `time.Date(...)` truncation to UTC midnight, the `map[string]struct{}` membership check, why `parsePartitionDate` returns `(time.Time, bool)` instead of `(time.Time, error)`, and the stable sort at the end. If anything is unclear — ask me, that's why I'm here.

### 3. Run the planner tests

```bash
go test ./internal/partitions/...
```

Expect 4 PASS. These run without Postgres because the planner is pure logic.

### 4. Read `maintainer.go`

Pay attention to:

- The `pg_inherits` query — this is the canonical "list partitions of X" pattern in PostgreSQL.
- Why we cannot use `$1` placeholders for table names in DDL.
- The `format('CREATE TABLE %I PARTITION OF %I ...', ...)` trick — Postgres' own `%I` and `%L` escape identifiers and literals safely, so we round-trip the SQL through the server to build a safe statement.

### 5. Apply migration 3

```bash
make migrate-up
make psql
\dt
```

You should see `outbox` and `event_aggregates` tables.

### 6. Run the partition worker

```bash
make run-partition-worker
```

Expect log lines like `"partition maintenance" op=create name=events_<future date>`. Then in another terminal:

```bash
make psql
\d+ events
```

You should see partitions extending 14 days into the future. If you re-run the worker, it should be a no-op (no `partition maintenance` lines) — that's the idempotency check working.

### 7. **Your task: write `partition_worker_test.go` (integration test)**

This is the code I want you to write yourself. Specification:

- File: `internal/partitions/maintainer_integration_test.go`
- Build tag: `//go:build integration` at the top so it only runs under `make test-integration`
- Connects to Postgres using the env var `PULSE_TEST_DSN` (skip the test if unset)
- The test should:
  1. Create a temporary parent table `test_events_<random>` partitioned by RANGE(created_at)
  2. Construct a `Maintainer` pointing at that table with `FutureDays=5`
  3. Inject a fixed `now()` (you'll need to expose a way — see hint below)
  4. Call `Tick(ctx)`
  5. Query `pg_inherits` and assert exactly 5 partitions exist with the expected names
  6. Call `Tick` again — assert no new partitions are created (idempotency)
  7. Drop the parent table at the end

Hints:

- The `Maintainer.now` field is unexported on purpose. You have two options: add a small constructor for tests like `NewMaintainerWithClock(pool, cfg, clock func() time.Time)`, or add a `WithClock` option. Pick one and justify it in the commit message.
- Use `t.Cleanup(...)` to drop the test table.
- Connection pool: `pgxpool.New(ctx, dsn)` is fine — no need for the full `internal/db` plumbing.

When the test passes locally, push and tag.

### 8. Update CI

Open `.github/workflows/ci.yml`. The `integration tests` step already runs `go test -tags=integration ./...` — but it doesn't set `PULSE_TEST_DSN`. Fix that by adding the same DSN as `POSTGRES_DSN` to the `env:` block of the test job.

### 9. Update `docs/architecture.md`

Add ADR-005: "Partition worker as its own process". Cover: why separate from ingest-api, what the trade-offs are, and what would change your mind. Two short paragraphs are enough.

### 10. Commit & tag

```bash
git add .
git commit -m "feat: partition maintenance worker; outbox + aggregates tables"
git tag week-2
git push --tags
git push
```

## Acceptance for this week

- `go test ./...` passes (unit + your new integration test)
- `golangci-lint run ./...` is clean
- `cmd/partition-worker` runs and is idempotent
- ADR-005 written
- `git tag week-2` pushed

## Stretch (only if you have time)

- Add a `--dry-run` flag to `partition-worker` that prints the plan without applying it. This is the kind of operational nicety that will matter in week 9 demos.
- Add a Prometheus counter `partitions_created_total` and `partitions_dropped_total`. Wiring up the metric server is part of week 8, but if you want to get there early go ahead.
