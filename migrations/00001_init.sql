-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Parent table, partitioned by created_at on day boundaries.
-- Daily partitions are created in 00002_partitions.sql and (in production)
-- maintained by a background worker that keeps a rolling 30-day window of
-- future partitions plus a default catch-all.
CREATE TABLE events (
    id          uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id     text        NOT NULL,
    event_type  text        NOT NULL,
    properties  jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- Partition key must be part of the primary key in declarative
    -- partitioning. id alone is not enough; we include created_at.
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

COMMENT ON TABLE events IS
    'Behavioral events. Range-partitioned daily; retention via partition drop in week 5.';

-- Default catch-all so inserts outside the rolling window do not fail.
-- In week 8 we'll alert on rows landing here.
CREATE TABLE events_default PARTITION OF events DEFAULT;

-- Indexes are intentionally NOT created here. Week 3 starts with EXPLAIN
-- ANALYZE baselines on each query pattern, then introduces indexes one at
-- a time and records the impact in docs/perf-log.md. Adding indexes blindly
-- before measuring would defeat the purpose of the exercise.
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS events CASCADE;
