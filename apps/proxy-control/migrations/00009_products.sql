-- +goose Up
CREATE TABLE product_sales (
    kind text PRIMARY KEY CHECK (kind IN ('dynamic', 'static')),
    open boolean NOT NULL DEFAULT false,
    quote_ready boolean NOT NULL DEFAULT false,
    acceptance_evidence text NOT NULL DEFAULT '',
    CHECK (NOT open OR (quote_ready AND acceptance_evidence <> ''))
);

CREATE TABLE traffic_packages (
    id text PRIMARY KEY,
    capacity_bytes bigint NOT NULL CHECK (capacity_bytes > 0),
    price_minor bigint NOT NULL DEFAULT 0 CHECK (price_minor >= 0),
    listed boolean NOT NULL DEFAULT true,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE product_change_log (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    target_kind text NOT NULL CHECK (target_kind IN ('sale', 'package')),
    target_id text NOT NULL,
    operator_id text NOT NULL REFERENCES operators(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    before_value jsonb NOT NULL,
    after_value jsonb NOT NULL
);

INSERT INTO product_sales (kind) VALUES ('dynamic'), ('static');
INSERT INTO traffic_packages (id, capacity_bytes) VALUES
    ('5gb', 5000000000), ('50gb', 50000000000), ('200gb', 200000000000);

-- +goose Down
DROP TABLE product_change_log;
DROP TABLE traffic_packages;
DROP TABLE product_sales;
