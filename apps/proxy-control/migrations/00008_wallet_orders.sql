-- +goose Up
CREATE TABLE purchase_orders (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES wallets(tenant_id),
    kind text NOT NULL CHECK (kind IN ('traffic_package', 'static_purchase', 'static_renewal')),
    status text NOT NULL CHECK (status IN ('pending', 'succeeded', 'partial', 'failed')),
    locked_minor bigint NOT NULL CHECK (locked_minor > 0),
    debited_minor bigint NOT NULL CHECK (debited_minor = locked_minor),
    released_minor bigint NOT NULL CHECK (released_minor BETWEEN 0 AND debited_minor),
    refunded_minor bigint NOT NULL CHECK (refunded_minor BETWEEN 0 AND debited_minor - released_minor),
    delivered_bytes bigint NOT NULL CHECK (delivered_bytes >= 0),
    delivered_count bigint NOT NULL CHECK (delivered_count >= 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, id)
);

CREATE INDEX purchase_orders_tenant_recent ON purchase_orders (tenant_id, id DESC);

CREATE TABLE order_items (
    tenant_id text NOT NULL,
    order_id bigint NOT NULL,
    position integer NOT NULL CHECK (position > 0),
    specification text NOT NULL,
    reference text NOT NULL,
    capacity_bytes bigint NOT NULL CHECK (capacity_bytes >= 0),
    term_days integer NOT NULL CHECK (term_days >= 0),
    unit_minor bigint NOT NULL CHECK (unit_minor > 0),
    quantity integer NOT NULL CHECK (quantity > 0),
    delivered_count integer NOT NULL CHECK (delivered_count BETWEEN 0 AND quantity),
    PRIMARY KEY (tenant_id, order_id, position),
    FOREIGN KEY (tenant_id, order_id) REFERENCES purchase_orders (tenant_id, id)
);

CREATE TABLE wallet_entries (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES wallets(tenant_id),
    request_key text NOT NULL,
    request_hash text NOT NULL,
    order_id bigint,
    kind text NOT NULL CHECK (kind IN ('recharge', 'debit', 'hold', 'release', 'refund')),
    actor text REFERENCES operators(id),
    reason text NOT NULL,
    before_minor bigint NOT NULL CHECK (before_minor >= 0),
    after_minor bigint NOT NULL CHECK (after_minor >= 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, request_key),
    FOREIGN KEY (tenant_id, order_id) REFERENCES purchase_orders (tenant_id, id),
    CHECK ((kind = 'recharge' AND actor IS NOT NULL AND reason <> '' AND order_id IS NULL)
        OR (kind <> 'recharge' AND actor IS NULL AND order_id IS NOT NULL))
);

CREATE INDEX wallet_entries_tenant_recent ON wallet_entries (tenant_id, id DESC)
    WHERE before_minor <> after_minor;

CREATE TABLE wallet_recharge_quotes (
    id uuid PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES wallets(tenant_id),
    actor text NOT NULL REFERENCES operators(id),
    request_key text NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    reason text NOT NULL CHECK (reason <> ''),
    expected_before_minor bigint NOT NULL CHECK (expected_before_minor >= 0),
    expires_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE wallet_recharge_quotes, wallet_entries, order_items, purchase_orders;
