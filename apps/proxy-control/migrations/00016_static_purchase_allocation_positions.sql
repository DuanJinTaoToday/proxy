-- +goose Up
ALTER TABLE order_items
    ADD COLUMN static_region text NOT NULL DEFAULT '',
    ADD COLUMN static_ip_type text NOT NULL DEFAULT '';

ALTER TABLE static_ip_allocations
    ADD COLUMN order_item_position integer NOT NULL DEFAULT 1,
    ADD CONSTRAINT static_ip_allocations_order_item_fk
        FOREIGN KEY (tenant_id, order_id, order_item_position) REFERENCES order_items (tenant_id, order_id, position);

-- +goose Down
ALTER TABLE static_ip_allocations DROP COLUMN order_item_position;
ALTER TABLE order_items DROP COLUMN static_region, DROP COLUMN static_ip_type;
