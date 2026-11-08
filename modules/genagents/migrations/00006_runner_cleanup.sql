-- +goose Up
-- cleanup_requested_at is when an admin asked the runners to free space; the runner reports it did with its next
-- heartbeat, which sets last_cleanup_at and the bytes it freed.
ALTER TABLE runners
    ADD COLUMN cleanup_requested_at     timestamptz,
    ADD COLUMN last_cleanup_at          timestamptz,
    ADD COLUMN last_cleanup_freed_bytes bigint;

-- +goose Down
ALTER TABLE runners
    DROP COLUMN last_cleanup_freed_bytes,
    DROP COLUMN last_cleanup_at,
    DROP COLUMN cleanup_requested_at;
