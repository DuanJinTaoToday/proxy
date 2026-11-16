-- +goose Up
ALTER TABLE tenant_balances ADD COLUMN usage_version bigint NOT NULL DEFAULT 1 CHECK (usage_version > 0);
ALTER TABLE proxy_subaccounts ADD CONSTRAINT proxy_subaccounts_tenant_client_key UNIQUE (tenant_id, client_id);

ALTER TABLE usage_daily DROP CONSTRAINT usage_daily_pkey,
    ADD COLUMN client_id text NOT NULL,
    ADD PRIMARY KEY (tenant_id, gateway_id, usage_date, client_id),
    ADD FOREIGN KEY (tenant_id, client_id) REFERENCES proxy_subaccounts (tenant_id, client_id),
    ADD FOREIGN KEY (gateway_id) REFERENCES gateways(id);

ALTER TABLE usage_minutely DROP CONSTRAINT usage_minutely_pkey,
    ADD COLUMN client_id text NOT NULL,
    ADD PRIMARY KEY (tenant_id, gateway_id, usage_date, minute_start, client_id),
    ADD FOREIGN KEY (tenant_id, client_id) REFERENCES proxy_subaccounts (tenant_id, client_id),
    ADD FOREIGN KEY (gateway_id) REFERENCES gateways(id);

CREATE TABLE traffic_grants (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES tenant_balances(tenant_id),
    order_id bigint NOT NULL,
    order_position integer NOT NULL,
    purchased_bytes bigint NOT NULL CHECK (purchased_bytes > 0),
    debt_offset_bytes bigint NOT NULL CHECK (debt_offset_bytes BETWEEN 0 AND purchased_bytes),
    remaining_bytes bigint NOT NULL CHECK (remaining_bytes BETWEEN 0 AND purchased_bytes - debt_offset_bytes),
    granted_at timestamptz NOT NULL,
    UNIQUE (tenant_id, order_id, order_position),
    FOREIGN KEY (tenant_id, order_id, order_position) REFERENCES order_items (tenant_id, order_id, position)
);

CREATE INDEX traffic_grants_fifo ON traffic_grants (tenant_id, granted_at, id) WHERE remaining_bytes > 0;

-- +goose Down
DROP TABLE traffic_grants;
ALTER TABLE usage_daily DROP CONSTRAINT usage_daily_pkey, DROP COLUMN client_id,
    ADD PRIMARY KEY (tenant_id, gateway_id, usage_date),
    DROP CONSTRAINT usage_daily_gateway_id_fkey;
ALTER TABLE usage_minutely DROP CONSTRAINT usage_minutely_pkey, DROP COLUMN client_id,
    ADD PRIMARY KEY (tenant_id, gateway_id, usage_date, minute_start),
    DROP CONSTRAINT usage_minutely_gateway_id_fkey;
ALTER TABLE proxy_subaccounts DROP CONSTRAINT proxy_subaccounts_tenant_client_key;
ALTER TABLE tenant_balances DROP COLUMN usage_version;
