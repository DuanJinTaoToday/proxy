-- +goose Up
ALTER TABLE static_ip_exchanges
    ADD COLUMN unknown_at timestamptz,
    ADD COLUMN next_check_at timestamptz,
    ADD COLUMN dispatched_at timestamptz,
    ADD COLUMN history_snapshot text,
    ADD COLUMN resolution text CHECK (resolution IN ('executed', 'not_executed')),
    ADD COLUMN resolution_conflict boolean NOT NULL DEFAULT false,
    ADD COLUMN corrected boolean NOT NULL DEFAULT false;

UPDATE static_ip_exchanges SET unknown_at = updated_at, next_check_at = updated_at WHERE status = 'unknown';
CREATE INDEX static_ip_exchanges_due_recheck ON static_ip_exchanges (next_check_at, tenant_id, idempotency_key)
    WHERE next_check_at IS NOT NULL;
CREATE INDEX static_ip_exchanges_ops_queue ON static_ip_exchanges (unknown_at, tenant_id, idempotency_key)
    WHERE status = 'unknown' OR resolution_conflict;
CREATE INDEX static_ip_exchanges_tenant_history ON static_ip_exchanges (tenant_id, created_at DESC, idempotency_key DESC);

-- A late-confirmed exchange may exhaust the quota after operations released its reservation.
-- Keep usage truthful in that case. New reservations and quota reductions still protect the limit.
ALTER TABLE static_ip_exchange_quotas DROP CONSTRAINT static_ip_exchange_quotas_check;
ALTER TABLE static_ip_exchange_quotas ADD CONSTRAINT static_ip_exchange_quotas_nonnegative
    CHECK (quota >= 0 AND used >= 0 AND reserved >= 0);

CREATE TABLE static_ip_exchange_audit (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES tenants(id),
    idempotency_key text,
    request_key text,
    actor_id text REFERENCES operators(id),
    kind text NOT NULL CHECK (kind IN ('adjustment', 'automatic', 'manual', 'duplicate', 'conflict', 'correction', 'observation')),
    evidence text,
    outcome text CHECK (outcome IN ('executed', 'not_executed')),
    supplier_record_id text,
    observed_ip text,
    quota_before integer NOT NULL,
    reserved_before integer NOT NULL,
    used_before integer NOT NULL,
    quota_after integer NOT NULL,
    reserved_after integer NOT NULL,
    used_after integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY (tenant_id, idempotency_key) REFERENCES static_ip_exchanges (tenant_id, idempotency_key)
);

CREATE INDEX static_ip_exchange_audit_tenant_history ON static_ip_exchange_audit (tenant_id, id DESC);
CREATE UNIQUE INDEX static_ip_exchange_audit_request ON static_ip_exchange_audit (tenant_id, request_key)
    WHERE request_key IS NOT NULL;
CREATE UNIQUE INDEX static_ip_exchange_audit_auto_receipt ON static_ip_exchange_audit (supplier_record_id)
    WHERE kind = 'automatic' AND supplier_record_id IS NOT NULL;

-- +goose Down
DROP TABLE static_ip_exchange_audit;
DROP INDEX static_ip_exchanges_due_recheck, static_ip_exchanges_ops_queue, static_ip_exchanges_tenant_history;
ALTER TABLE static_ip_exchanges
    DROP COLUMN unknown_at, DROP COLUMN next_check_at, DROP COLUMN dispatched_at, DROP COLUMN history_snapshot,
    DROP COLUMN resolution, DROP COLUMN resolution_conflict, DROP COLUMN corrected;
ALTER TABLE static_ip_exchange_quotas DROP CONSTRAINT static_ip_exchange_quotas_nonnegative;
ALTER TABLE static_ip_exchange_quotas ADD CONSTRAINT static_ip_exchange_quotas_check
    CHECK (used >= 0 AND reserved >= 0 AND quota - used >= reserved);
