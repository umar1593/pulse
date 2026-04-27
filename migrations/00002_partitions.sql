-- +goose Up
-- +goose StatementBegin
-- Create 14 daily partitions: yesterday, today, and the next 12 days.
-- A maintenance routine (see internal/partitions in week 2) keeps this
-- rolling forward; this migration just gets us off the ground.
DO $$
DECLARE
    d        date;
    start_d  date := current_date - INTERVAL '1 day';
    name_pat text;
BEGIN
    FOR i IN 0..13 LOOP
        d := start_d + (i * INTERVAL '1 day');
        name_pat := format('events_%s', to_char(d, 'YYYYMMDD'));
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF events FOR VALUES FROM (%L) TO (%L)',
            name_pat, d, d + INTERVAL '1 day'
        );
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    d        date;
    start_d  date := current_date - INTERVAL '1 day';
    name_pat text;
BEGIN
    FOR i IN 0..13 LOOP
        d := start_d + (i * INTERVAL '1 day');
        name_pat := format('events_%s', to_char(d, 'YYYYMMDD'));
        EXECUTE format('DROP TABLE IF EXISTS %I', name_pat);
    END LOOP;
END $$;
-- +goose StatementEnd
