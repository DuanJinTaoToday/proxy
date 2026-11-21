-- +goose Up
ALTER TABLE proxy_subaccounts ADD COLUMN created_at timestamptz;
ALTER TABLE proxy_subaccounts ALTER COLUMN created_at SET DEFAULT clock_timestamp();

-- +goose Down
ALTER TABLE proxy_subaccounts DROP COLUMN created_at;
