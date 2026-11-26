-- +goose Up
ALTER TABLE wallet_entries
    DROP CONSTRAINT wallet_entries_kind_check,
    DROP CONSTRAINT wallet_entries_check,
    ADD CONSTRAINT wallet_entries_kind_check
        CHECK (kind IN ('recharge', 'commission_transfer', 'debit', 'hold', 'release', 'refund')),
    ADD CONSTRAINT wallet_entries_check
        CHECK ((kind = 'recharge' AND actor IS NOT NULL AND reason <> '' AND order_id IS NULL)
            OR (kind = 'commission_transfer' AND actor IS NULL AND reason = '' AND order_id IS NULL AND after_minor > before_minor)
            OR (kind NOT IN ('recharge', 'commission_transfer') AND actor IS NULL AND order_id IS NOT NULL));

-- +goose Down
-- Restoring the previous check must fail if any transfer has posted. Keep the
-- ledger and financial balances intact in that case.
ALTER TABLE wallet_entries
    DROP CONSTRAINT wallet_entries_kind_check,
    DROP CONSTRAINT wallet_entries_check,
    ADD CONSTRAINT wallet_entries_kind_check
        CHECK (kind IN ('recharge', 'debit', 'hold', 'release', 'refund')),
    ADD CONSTRAINT wallet_entries_check
        CHECK ((kind = 'recharge' AND actor IS NOT NULL AND reason <> '' AND order_id IS NULL)
            OR (kind <> 'recharge' AND actor IS NULL AND order_id IS NOT NULL));
