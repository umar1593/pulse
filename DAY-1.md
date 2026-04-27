# Day 1 — get to a green run

Follow these steps in order. Stop at the first one that fails and fix it; don't skip ahead. Time budget: ~2 hours including tooling install.

## 0. Prerequisites (install once)

- Go 1.23+ — `go version`
- Docker + Docker Compose — `docker compose version`
- Python 3.12+ (only needed for week 9 loadgen) — `python3 --version`
- `goose` migrator — `go install github.com/pressly/goose/v3/cmd/goose@latest`
- `golangci-lint` — install per the [official instructions](https://golangci-lint.run/welcome/install/)

## 1. Move the project into your own location

The skeleton lives in your working folder. Copy or move it to a permanent place — somewhere like `~/projects/pulse`.

## 2. Replace the module path

In `go.mod` and every `.go` file, replace `github.com/youruser/pulse` with your real module path (your GitHub username). One command:

```bash
grep -rl 'github.com/youruser/pulse' . | xargs sed -i '' 's#github.com/youruser/pulse#github.com/<YOURUSER>/pulse#g'
```

(`sed -i ''` is for macOS; on Linux drop the empty quotes.)

## 3. Initialize git

```bash
git init
git add .
git commit -m "scaffold: ingest-api skeleton with partitioned events"
```

Push to a fresh public GitHub repo. The repo URL goes on your CV.

## 4. Pull dependencies

```bash
go mod tidy
```

This generates `go.sum` and downloads pgx, chi, uuid.

## 5. Bring up Postgres

```bash
cp .env.example .env
make up
make logs        # in another terminal — wait until you see "ready to accept connections"
```

## 6. Run migrations

```bash
make migrate-up
make migrate-status     # all migrations should be in `Applied`
```

Check the partitions exist:

```bash
make psql
\d+ events
```

You should see 14 daily partitions plus `events_default`.

## 7. Run the service

```bash
make run
```

Expect a JSON log line `"ingest-api listening" addr=":8080"`.

## 8. Smoke test

In another terminal:

```bash
curl -sS -i http://localhost:8080/healthz
curl -sS -i http://localhost:8080/readyz
curl -sS -i -X POST http://localhost:8080/events \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"u-1","event_type":"page_view","properties":{"path":"/pricing"}}'
```

The third call should return `202 Accepted`. Then:

```bash
make psql
SELECT id, user_id, event_type, properties, created_at
FROM events
ORDER BY created_at DESC
LIMIT 5;
```

Confirm the row is there. Confirm it landed in today's partition:

```sql
SELECT tableoid::regclass AS partition, count(*)
FROM events
GROUP BY 1
ORDER BY 1;
```

## 9. Run tests

```bash
make test
make lint
```

Lint should be clean. Tests should be green.

## 10. Push and tag

```bash
git add .
git commit -m "chore: tidy modules, day 1 verified"
git push
git tag week-1
git push --tags
```

## What you've demonstrated by end of day 1

- A running Go service with pgx pool, chi router, structured logging, graceful shutdown
- A range-partitioned PostgreSQL table with daily granularity
- Migrations via goose, running both forward and backward
- A tuned `postgresql.conf` with `pg_stat_statements` and `auto_explain` enabled
- Unit tests, lint, and a CI workflow ready to go green when you push

That is already more than most candidates show on day one.

## What's next

Open `docs/roadmap.md` and start week 2. The first task there is the partition-maintenance worker.

---

If you hit any error in steps 1–9, paste the output and I'll command the fix.
