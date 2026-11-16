-- +goose Up
CREATE TABLE static_renewal_items (
    tenant_id text NOT NULL,
    order_id bigint NOT NULL,
    position integer NOT NULL,
    supplier_item_id text NOT NULL REFERENCES static_ip_allocations(supplier_item_id),
    supplier_order_id text NOT NULL,
    original_ip text NOT NULL,
    original_expiry timestamptz NOT NULL,
    expected_expiry timestamptz NOT NULL CHECK (expected_expiry > original_expiry),
    outcome text NOT NULL DEFAULT 'pending' CHECK (outcome IN ('pending', 'succeeded', 'failed')),
    PRIMARY KEY (tenant_id, order_id, position),
    FOREIGN KEY (tenant_id, order_id, position) REFERENCES order_items(tenant_id, order_id, position)
);

CREATE UNIQUE INDEX static_renewal_items_pending_allocation
    ON static_renewal_items (supplier_item_id) WHERE outcome = 'pending';

CREATE TABLE static_renewal_resolutions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL,
    order_id bigint NOT NULL,
    position integer NOT NULL,
    operator_id text NOT NULL REFERENCES operators(id),
    evidence text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY (tenant_id, order_id, position) REFERENCES static_renewal_items(tenant_id, order_id, position)
);

-- +goose Down
DROP TABLE static_renewal_resolutions, static_renewal_items;
