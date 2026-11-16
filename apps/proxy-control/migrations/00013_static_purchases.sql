-- +goose Up
CREATE TABLE static_purchases (
    tenant_id text NOT NULL,
    order_id bigint NOT NULL,
    supplier_order_id text UNIQUE,
    PRIMARY KEY (tenant_id, order_id),
    FOREIGN KEY (tenant_id, order_id) REFERENCES purchase_orders (tenant_id, id)
);

CREATE TABLE static_ip_allocations (
    supplier_item_id text PRIMARY KEY,
    tenant_id text NOT NULL,
    order_id bigint NOT NULL,
    ip text NOT NULL,
    port integer NOT NULL CHECK (port BETWEEN 1 AND 65535),
    username text NOT NULL,
    password_ciphertext bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    reminded_expiry timestamptz,
    FOREIGN KEY (tenant_id, order_id) REFERENCES static_purchases (tenant_id, order_id)
);
CREATE INDEX static_ip_allocations_by_tenant ON static_ip_allocations (tenant_id, expires_at, supplier_item_id);

CREATE TABLE static_purchase_resolutions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL,
    order_id bigint NOT NULL,
    operator_id text NOT NULL REFERENCES operators(id),
    evidence text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY (tenant_id, order_id) REFERENCES static_purchases (tenant_id, order_id)
);

-- +goose Down
DROP TABLE static_purchase_resolutions, static_ip_allocations, static_purchases;
