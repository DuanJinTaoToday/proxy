-- +goose Up
ALTER TABLE static_ip_allocations
    ADD COLUMN proxy_id text NOT NULL DEFAULT '';

CREATE TABLE static_ip_exchange_quotas (
    tenant_id text PRIMARY KEY REFERENCES tenants(id),
    quota integer NOT NULL DEFAULT 10,
    used integer NOT NULL DEFAULT 0,
    reserved integer NOT NULL DEFAULT 0,
    CHECK (used >= 0 AND reserved >= 0 AND quota - used >= reserved)
);

CREATE TABLE static_ip_exchanges (
    tenant_id text NOT NULL,
    idempotency_key text NOT NULL,
    allocation_id text NOT NULL,
    node_id bigint NOT NULL CHECK (node_id > 0),
    status text NOT NULL CHECK (status IN ('pending', 'unknown', 'failed', 'activating', 'ready', 'support_required')),
    supplier_order_id text NOT NULL,
    supplier_item_id text NOT NULL,
    original_proxy_id text NOT NULL,
    original_ip text NOT NULL,
    original_port integer NOT NULL,
    original_username text NOT NULL,
    original_password_ciphertext bytea NOT NULL,
    original_expiry timestamptz NOT NULL,
    new_proxy_id text,
    new_ip text,
    probe_attempts integer NOT NULL DEFAULT 0 CHECK (probe_attempts >= 0),
    probe_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, idempotency_key),
    FOREIGN KEY (tenant_id, allocation_id) REFERENCES static_ip_allocations (tenant_id, allocation_id)
);

CREATE UNIQUE INDEX static_ip_exchanges_in_progress ON static_ip_exchanges (allocation_id)
    WHERE status IN ('pending', 'unknown', 'activating');
CREATE INDEX static_ip_exchanges_by_allocation ON static_ip_exchanges (allocation_id, created_at DESC);
CREATE INDEX static_ip_exchanges_probe_due ON static_ip_exchanges (probe_at)
    WHERE status = 'activating';

-- +goose Down
DROP TABLE static_ip_exchanges, static_ip_exchange_quotas;
ALTER TABLE static_ip_allocations DROP COLUMN proxy_id;
