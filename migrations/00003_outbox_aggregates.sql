-- +goose Up
-- +goose StatementBegin

-- Outbox: a transactional queue of work that the aggregator (week 5) will
-- pick up via SELECT ... FOR UPDATE SKIP LOCKED. Inserts into `events` and
-- `outbox` happen in the same transaction so we never lose work even if the
-- aggregator is down.
CREATE TABLE outbox (
    id           bigserial   PRIMARY KEY,
    payload      jsonb       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    locked_by    text,
    locked_at    timestamptz,
    processed_at timestamptz
);

-- Pending work index: only un-processed rows. Partial index keeps it tiny
-- regardless of how big `outbox` grows historically.
CREATE INDEX outbox_pending_idx
    ON outbox (created_at)
    WHERE processed_at IS NULL;

COMMENT ON TABLE outbox IS
    'Transactional outbox. Aggregator claims rows with FOR UPDATE SKIP LOCKED.';

-- Pre-aggregated event counts. Filled by the aggregator (week 5). Kept
-- separate from `events` so it can be reindexed/rebuilt without touching
-- the hot ingestion table.
CREATE TABLE event_aggregates (
    bucket_start timestamptz NOT NULL,   -- start of the hour (UTC)
    user_id      text        NOT NULL,
    event_type   text        NOT NULL,
    event_count  bigint      NOT NULL,
    PRIMARY KEY (bucket_start, user_id, event_type)
);

COMMENT ON TABLE event_aggregates IS
    'Hourly per-user per-event-type counts. Populated by aggregator (week 5).';

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS event_aggregates;
DROP TABLE IF EXISTS outbox;
