-- +goose Up
CREATE TABLE mock_supplier_usage (
    supplier_account_id text NOT NULL,
    record_id text NOT NULL,
    tenant_id text NOT NULL,
    usage_date date NOT NULL,
    total_bytes bigint NOT NULL CHECK (total_bytes > 0),
    PRIMARY KEY (supplier_account_id, record_id)
);
CREATE INDEX mock_supplier_usage_tenant_date ON mock_supplier_usage (tenant_id, usage_date);

-- +goose Down
DROP TABLE mock_supplier_usage;
