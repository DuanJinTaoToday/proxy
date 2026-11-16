-- +goose Up
ALTER TABLE tenant_balances
    ADD COLUMN low_balance_threshold_bytes bigint CHECK (low_balance_threshold_bytes >= 0),
    ADD COLUMN low_balance_cycle bigint NOT NULL DEFAULT 0 CHECK (low_balance_cycle >= 0);

CREATE TABLE tenant_notifications (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES tenants(id),
    kind text NOT NULL,
    subject_key text NOT NULL,
    occurrence bigint NOT NULL CHECK (occurrence > 0),
    created_at timestamptz NOT NULL,
    read_at timestamptz,
    resolved_at timestamptz,
    UNIQUE (tenant_id, kind, subject_key, occurrence)
);

CREATE UNIQUE INDEX tenant_notifications_active
    ON tenant_notifications (tenant_id, kind, subject_key) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE tenant_notifications;
ALTER TABLE tenant_balances DROP COLUMN low_balance_cycle, DROP COLUMN low_balance_threshold_bytes;
