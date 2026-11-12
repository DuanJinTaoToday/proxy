package server_test

import (
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

func TestHistoricalGroupRateAndExpiredGrant(t *testing.T) {
	for _, expiryFirst := range []bool{false, true} {
		name := "ingest-before-expiry-settlement"
		if expiryFirst {
			name = "ingest-after-expiry-settlement"
		}
		t.Run(name, func(t *testing.T) {
			f := setupUsage(t)
			execSQL(t, f.db, `INSERT INTO tenant_groups (id, name) VALUES ('group-a', 'A'), ('group-b', 'B');
				INSERT INTO tenant_group_members (tenant_id, effective_from, group_id, updated_by)
				VALUES ('alpha', '2020-11-10T00:00:00+08:00', 'group-a', 'test'),
				       ('alpha', '2020-11-11T00:00:00+08:00', 'group-b', 'test');
				INSERT INTO billing_rates (scope_type, scope_id, upload_ratio_milli, download_ratio_milli, effective_from, created_by)
				VALUES ('group', 'group-a', 1200, 1000, '2020-11-10T00:00:00+08:00', 'test'),
				       ('group', 'group-b', 2000, 1000, '2020-11-10T00:00:00+08:00', 'test');
				INSERT INTO traffic_grants (tenant_id, granted_bytes, starts_at, expires_at, source, created_by)
				VALUES ('alpha', 100, '2020-11-10T00:00:00+08:00', '2020-11-11T00:00:00+08:00', 'recharge', 'test');`)
			settle := func() {
				t.Helper()
				if err := f.service.SettleGrants(t.Context(), time.Date(2020, 11, 12, 0, 0, 0, 0, time.UTC)); err != nil {
					t.Fatal(err)
				}
			}
			if expiryFirst {
				settle()
			}
			// 25 * 1.2 = 30 against the expired grant. The next record uses
			// group B and goes to overdraft because it falls outside the grant.
			first := recorder(0, "client-a", beforeMidnight, 25, 0, 25, 0)
			second := recorder(first.GetEndOffset(), "client-a", afterMidnight, 35, 0, 10, 0)
			submit(t, f, "gateway-a", reportToken, first, second)
			settle()
			settle()
			assertDays(t, daily(t, f, "alpha", f.alice), usageDay("2020-11-10", 25, 0, 30), usageDay("2020-11-11", 10, 0, 20))
			var ledger struct{ Net, Expiries, Corrections int64 }
			if err := f.db.Raw(`SELECT SUM(delta_bytes) AS net,
				count(*) FILTER (WHERE kind = 'grant_expire') AS expiries,
				count(*) FILTER (WHERE kind = 'grant_expire_correction') AS corrections
				FROM balance_ledger WHERE tenant_id = 'alpha'`).Scan(&ledger).Error; err != nil {
				t.Fatal(err)
			}
			var expectedCorrections int64
			if expiryFirst {
				expectedCorrections = 1
			}
			if ledger.Net != -70 || ledger.Expiries != 1 || ledger.Corrections != expectedCorrections {
				t.Fatalf("unexpected expiry reconciliation: %+v", ledger)
			}
		})
	}
}

func TestUsageFailedWriteCanBeRetried(t *testing.T) {
	f := setupUsage(t)
	first := recorder(0, "client-a", beforeMidnight, 130, 214, 130, 214)
	// Reject cursor insertion after the transaction has written usage and balances.
	execSQL(t, f.db, "ALTER TABLE ingest_cursors ADD CONSTRAINT injected_failure CHECK (confirmed_offset = 0)")
	reply := submitResponse(t, f, "gateway-a", reportToken, first)
	if reply.Code != 500 {
		t.Fatalf("expected transaction failure, got %d %s", reply.Code, reply.Body)
	}
	if got := daily(t, f, "alpha", f.alice); len(got.Days) != 0 || got.LastReceivedAt != nil {
		t.Fatalf("failed transaction changed usage: %v", got)
	}
	execSQL(t, f.db, "ALTER TABLE ingest_cursors DROP CONSTRAINT injected_failure")
	if got := submit(t, f, "gateway-a", reportToken, first); got.GetBillable() != 1 || got.GetDuplicates() != 0 {
		t.Fatalf("retry was not accepted as new usage: %v", got)
	}
	assertDays(t, daily(t, f, "alpha", f.alice), usageDay("2020-11-10", 130, 214, 344))
}

func TestMissedGrantWindowDoesNotOffsetCurrentDebt(t *testing.T) {
	f := setupUsage(t)
	afterExpiry := recorder(0, "client-a", afterMidnight, 20, 0, 20, 0)
	submit(t, f, "gateway-a", reportToken, afterExpiry)
	execSQL(t, f.db, `INSERT INTO traffic_grants (tenant_id, granted_bytes, starts_at, expires_at, source, created_by)
		VALUES ('alpha', 100, '2020-11-10T00:00:00+08:00', '2020-11-11T00:00:00+08:00', 'recharge', 'test')`)
	late := recorder(0, "client-a", beforeMidnight, 10, 0, 10, 0)
	late.FileId = "historical"
	submit(t, f, "gateway-a", reportToken, late)
	var balance v1.TenantBalance
	readProto(t, request(f.http, "GET", "/v1/tenants/alpha/balance", "", f.alice), &balance)
	if balance.GetOverdraftBytes() != 20 || balance.GetAvailableBytes() != -20 || len(balance.Grants) != 0 {
		t.Fatalf("expired grant changed current debt: %v", &balance)
	}
	var grant struct{ UsedBytes, OverdraftOffsetBytes int64 }
	if err := f.db.Table("traffic_grants").Take(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if grant.UsedBytes != 10 || grant.OverdraftOffsetBytes != 0 {
		t.Fatalf("expired grant did not allocate the historical sample: %+v", grant)
	}
}
