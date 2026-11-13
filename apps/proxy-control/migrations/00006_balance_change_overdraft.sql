-- +goose Up
ALTER TABLE balance_ledger ADD COLUMN overdraft_added_bytes bigint
    CHECK (overdraft_added_bytes IS NULL OR (overdraft_added_bytes >= 0 AND kind = 'adjust_subtract'));

-- +goose Down
ALTER TABLE balance_ledger DROP COLUMN overdraft_added_bytes;
