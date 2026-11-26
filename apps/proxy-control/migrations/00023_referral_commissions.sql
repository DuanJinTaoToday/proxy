-- +goose Up
ALTER TABLE purchase_orders ADD COLUMN settled_at timestamptz;
ALTER TABLE purchase_orders ADD CONSTRAINT purchase_orders_pending_not_settled
    CHECK (status <> 'pending' OR settled_at IS NULL);

CREATE TABLE commission_rate_versions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    first_purchase_bps integer NOT NULL CHECK (first_purchase_bps BETWEEN 0 AND 10000),
    repeat_purchase_bps integer NOT NULL CHECK (repeat_purchase_bps BETWEEN 0 AND 10000),
    operator_id text REFERENCES operators(id),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE commission_rate_settings (
    id integer PRIMARY KEY CHECK (id = 1),
    version_id bigint NOT NULL REFERENCES commission_rate_versions(id)
);

WITH initial_rates AS (
    INSERT INTO commission_rate_versions (first_purchase_bps, repeat_purchase_bps)
    VALUES (0, 0) RETURNING id
)
INSERT INTO commission_rate_settings SELECT 1, id FROM initial_rates;

CREATE TABLE commission_accounts (
    tenant_id text PRIMARY KEY REFERENCES tenants(id),
    available_minor bigint NOT NULL DEFAULT 0 CHECK (available_minor >= 0)
);

CREATE TABLE referral_commissions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id bigint NOT NULL UNIQUE,
    buyer_tenant_id text NOT NULL,
    referrer_tenant_id text NOT NULL REFERENCES commission_accounts(tenant_id),
    customer_key uuid NOT NULL,
    referral_source text NOT NULL CHECK (referral_source IN ('manual', 'link')),
    product_kind text NOT NULL CHECK (product_kind IN ('traffic_package', 'static_purchase', 'static_renewal')),
    settlement_sequence bigint NOT NULL CHECK (settlement_sequence > 0),
    net_debit_minor bigint NOT NULL CHECK (net_debit_minor > 0),
    commission_minor bigint NOT NULL CHECK (commission_minor BETWEEN 0 AND net_debit_minor),
    rate_bps integer NOT NULL CHECK (rate_bps BETWEEN 0 AND 10000),
    rate_version bigint NOT NULL REFERENCES commission_rate_versions(id),
    rule_version integer NOT NULL CHECK (rule_version > 0),
    promotion_paused boolean NOT NULL,
    settled_at timestamptz NOT NULL,
    UNIQUE (buyer_tenant_id, settlement_sequence),
    FOREIGN KEY (buyer_tenant_id, order_id) REFERENCES purchase_orders (tenant_id, id),
    CHECK (buyer_tenant_id <> referrer_tenant_id),
    CHECK (NOT promotion_paused OR (rate_bps = 0 AND commission_minor = 0))
);

CREATE INDEX referral_commissions_recent ON referral_commissions (referrer_tenant_id, id DESC);
CREATE INDEX referral_commissions_customer ON referral_commissions (referrer_tenant_id, customer_key, id DESC);

-- +goose Down
DROP TABLE referral_commissions, commission_accounts, commission_rate_settings, commission_rate_versions;
ALTER TABLE purchase_orders DROP COLUMN settled_at;
