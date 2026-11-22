INSERT INTO usage_gaps (gateway_id, boot_id, session_id, from_at, detected_at, resolved_at)
SELECT 'gw-pagination', 'paged', 'session-' || n, '2026-11-18 02:00+08'::timestamptz,
  '2026-11-18 02:02+08'::timestamptz + n * interval '1 second',
  CASE WHEN n % 2 = 0 THEN '2026-11-18 02:03+08'::timestamptz END
FROM generate_series(1, 43) n;
INSERT INTO record_anomalies (gateway_id, boot_id, file_id, file_offset, kind, detail, detected_at, resolved_at)
SELECT 'gw-pagination', 'paged', 'fixture-' || n, n, 'invalid_record', '{}',
  '2026-11-18 02:02+08'::timestamptz + n * interval '1 second',
  CASE WHEN n % 2 = 0 THEN '2026-11-18 02:03+08'::timestamptz END
FROM generate_series(1, 43) n;
