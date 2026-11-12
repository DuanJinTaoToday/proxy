-- +goose Up
ALTER TABLE gateway_boots ADD COLUMN last_heartbeat_at timestamptz, ADD COLUMN exited_at timestamptz;

CREATE VIEW gateway_boot_lifecycle AS
SELECT gateway_id, boot_id, last_heartbeat_at, exited_at,
    CASE
        WHEN exited_at IS NOT NULL THEN 'exited'
        WHEN COALESCE(last_heartbeat_at, first_seen_at) <= now() - interval '15 seconds' THEN 'disconnected'
        WHEN last_heartbeat_at IS NULL THEN 'unknown'
        ELSE 'running'
    END AS status
FROM gateway_boots;

-- +goose Down
DROP VIEW gateway_boot_lifecycle;
ALTER TABLE gateway_boots DROP COLUMN exited_at, DROP COLUMN last_heartbeat_at;
