WITH accounts (supplier_account_id, tenant_id) AS (
    VALUES ('mock-account-a', 'alpha'), ('mock-account-b', 'beta')
), samples (supplier_account_id, record_id, total_bytes) AS (
    VALUES ('mock-account-a', 'demo-1', 104857600),
           ('mock-account-b', 'demo-1', 209715200)
)
INSERT INTO mock_supplier_usage (supplier_account_id, record_id, tenant_id, usage_date, total_bytes)
SELECT samples.supplier_account_id, samples.record_id, accounts.tenant_id,
       (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date, samples.total_bytes
FROM samples
JOIN accounts USING (supplier_account_id)
ON CONFLICT (supplier_account_id, record_id) DO NOTHING;
