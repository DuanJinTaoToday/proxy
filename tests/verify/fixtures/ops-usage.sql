BEGIN;
INSERT INTO gateways (id, name) VALUES ('gw-sg-01', 'Singapore 01'), ('gw-sg-02', 'Singapore 02'), ('gw-pagination', 'Pagination');
INSERT INTO tenants (id, name, primary_account_name) VALUES ('t-1002', '诺瓦跨境', 'nova-trade'), ('t-1004', '映像科技', 'ying-data');
INSERT INTO proxy_subaccounts (client_id, tenant_id, name, password_ciphertext, active) VALUES
    ('verify-nova', 't-1002', 'nova', ''::bytea, false),
    ('verify-ying', 't-1004', 'ying', ''::bytea, false),
    ('verify-chenxl', 'visual-recharge-tenant', 'verify', ''::bytea, false);
INSERT INTO gateway_boots (gateway_id, boot_id, first_seen_at, last_record_at, exited_at, replay_completed_at) VALUES
    ('gw-sg-02', 'b-20261120-d7', '2026-11-20 05:00+08', '2026-11-20 06:40+08', '2026-11-20 06:42+08', NULL),
    ('gw-sg-01', 'b-20261119-a3', '2026-11-19 20:00+08', '2026-11-19 22:58+08', '2026-11-19 23:00+08', '2026-11-20 01:10+08'),
    ('gw-sg-01', 'b-20261117-f2', '2026-11-17 12:00+08', '2026-11-17 13:05+08', '2026-11-17 13:06+08', NULL),
    ('gw-pagination', 'paged', '2026-11-18 01:00+08', '2026-11-18 02:00+08', '2026-11-18 02:01+08', '2026-11-18 02:03+08');
INSERT INTO usage_gaps (gateway_id, boot_id, session_id, tenant_id, from_at, detected_at, resolved_at) VALUES
    ('gw-sg-01', 'b-20261119-a3', 's-8c1e44f0', 'visual-recharge-tenant', '2026-11-19 22:58+08', '2026-11-20 01:10+08', NULL),
    ('gw-sg-01', 'b-20261117-f2', 's-71d0a9b2', 't-1002', '2026-11-17 13:05+08', '2026-11-17 13:07+08', '2026-11-17 13:31+08');
INSERT INTO record_anomalies (gateway_id, boot_id, file_id, file_offset, kind, detail, detected_at, resolved_at) VALUES
    ('gw-sg-01', 'b-20261119-a3', 'rec-000412', 1048576, 'decode_error', '{"client_id":"verify-ying"}', '2026-11-20 03:12+08', NULL),
    ('gw-sg-02', 'b-20261118-c1', 'rec-000388', 524288, 'checksum_mismatch', '{"client_id":"verify-chenxl"}', '2026-11-19 17:46+08', NULL),
    ('gw-sg-01', 'b-20261117-f2', 'rec-000351', 2097152, 'truncated_record', '{"client_id":"verify-nova"}', '2026-11-18 08:20+08', '2026-11-18 09:05+08');
INSERT INTO operators (id, username, password_hash)
SELECT 'verify-no-read', 'verify-no-read', password_hash FROM operators WHERE id = 'local-operator-readonly';
COMMIT;
