-- +goose Up
ALTER TABLE static_ip_allocations
    ADD COLUMN allocation_id text NOT NULL DEFAULT ('sa_' || gen_random_uuid()::text),
    ADD CONSTRAINT static_ip_allocations_allocation_id_key UNIQUE (allocation_id),
    ADD CONSTRAINT static_ip_allocations_tenant_allocation_key UNIQUE (tenant_id, allocation_id);

ALTER TABLE static_renewal_items ADD COLUMN allocation_id text;

UPDATE static_renewal_items r
SET allocation_id = a.allocation_id
FROM static_ip_allocations a
WHERE a.tenant_id = r.tenant_id AND a.supplier_item_id = r.supplier_item_id;

UPDATE order_items i
SET reference = r.allocation_id
FROM static_renewal_items r
WHERE (i.tenant_id, i.order_id, i.position) = (r.tenant_id, r.order_id, r.position);

ALTER TABLE static_renewal_items
    ALTER COLUMN allocation_id SET NOT NULL,
    ADD CONSTRAINT static_renewal_items_allocation_fk
        FOREIGN KEY (tenant_id, allocation_id) REFERENCES static_ip_allocations (tenant_id, allocation_id);

ALTER TABLE static_renewal_items DROP COLUMN supplier_item_id;

CREATE UNIQUE INDEX static_renewal_items_pending_allocation
    ON static_renewal_items (allocation_id) WHERE outcome = 'pending';

-- +goose Down
ALTER TABLE static_renewal_items ADD COLUMN supplier_item_id text;

UPDATE static_renewal_items r
SET supplier_item_id = a.supplier_item_id
FROM static_ip_allocations a
WHERE a.tenant_id = r.tenant_id AND a.allocation_id = r.allocation_id;

UPDATE order_items i
SET reference = r.supplier_item_id
FROM static_renewal_items r
WHERE (i.tenant_id, i.order_id, i.position) = (r.tenant_id, r.order_id, r.position);

ALTER TABLE static_renewal_items
    ALTER COLUMN supplier_item_id SET NOT NULL,
    ADD CONSTRAINT static_renewal_items_supplier_item_id_fkey
        FOREIGN KEY (supplier_item_id) REFERENCES static_ip_allocations (supplier_item_id);

ALTER TABLE static_renewal_items DROP COLUMN allocation_id;

CREATE UNIQUE INDEX static_renewal_items_pending_allocation
    ON static_renewal_items (supplier_item_id) WHERE outcome = 'pending';

ALTER TABLE static_ip_allocations DROP COLUMN allocation_id;
