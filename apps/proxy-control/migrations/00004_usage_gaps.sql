-- +goose Up
CREATE TABLE usage_gaps (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    gateway_id text NOT NULL,
    boot_id text NOT NULL,
    session_id text,
    client_id text,
    tenant_id text,
    from_at timestamptz NOT NULL,
    detected_at timestamptz NOT NULL,
    resolved_at timestamptz,
    UNIQUE NULLS NOT DISTINCT (gateway_id, boot_id, session_id),
    FOREIGN KEY (gateway_id, boot_id) REFERENCES gateway_boots(gateway_id, boot_id)
);

-- +goose Down
DROP TABLE usage_gaps;
