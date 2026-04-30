-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX events_idempotency_idx
    ON events (
        user_id,
        event_type,
        (properties->>'idempotency_key'),
        created_at
    )
    WHERE properties ? 'idempotency_key';
-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS events_idempotency_idx;
