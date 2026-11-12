package integration

import (
	"net/url"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"google.golang.org/protobuf/proto"
)

func TestOperationsBillingReconciliation(t *testing.T) {
	f := setupUsage(t)
	operator := testfixture.Login(t, f.http, "operator")
	start := time.Now().UTC().Truncate(24 * time.Hour).Add(-22 * time.Hour)
	rateSwitch := start.Add(30 * time.Second)
	execSQL(t, f.db, `INSERT INTO billing_rates (scope_type, scope_id, upload_ratio_milli, download_ratio_milli, effective_from, created_by)
		VALUES ('tenant', 'alpha', 2000, 1000, ?, 'test')`, rateSwitch)

	addGrant := func(bytes int64, at time.Time, ref string) {
		t.Helper()
		execSQL(t, f.db, `WITH inserted_grant AS (
			INSERT INTO traffic_grants (tenant_id, granted_bytes, starts_at, source, created_by)
			VALUES ('alpha', ?, ?, 'recharge', 'test') RETURNING id
		)
		INSERT INTO balance_ledger (tenant_id, kind, grant_id, delta_bytes, available_before, available_after, external_ref, operator, created_at)
		SELECT 'alpha', 'recharge', id, ?, COALESCE(b.overdraft_bytes, 0) * -1,
			? - COALESCE(b.overdraft_bytes, 0), ?, 'test', ? FROM inserted_grant
		LEFT JOIN tenant_balances b ON b.tenant_id = 'alpha'`, bytes, at, bytes, bytes, ref, at)
	}
	addGrant(50, start, "first-grant")
	first := recorder(0, "client-a", start.Add(time.Second).Format(time.RFC3339), 30, 0, 30, 0)
	second := recorder(first.GetEndOffset(), "client-a", rateSwitch.Format(time.RFC3339), 30, 0, 30, 0)
	if got := submit(t, f, "gateway-a", reportToken, first, second).GetBillable(); got != 2 {
		t.Fatalf("billable records = %d, want 2", got)
	}
	addGrant(25, start.Add(time.Minute), "offset-grant")
	// The ordinary balance read activates the new grant and writes the offset entry.
	var balance v1.TenantBalance
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenants/alpha/balance", "", f.alice), &balance)
	if balance.GetOverdraftBytes() != 15 {
		t.Fatalf("overdraft after grant activation = %d, want 15", balance.GetOverdraftBytes())
	}

	read := func(path string, target proto.Message) {
		t.Helper()
		readProto(t, testfixture.Request(f.http, "GET", path, "", operator), target)
	}
	const grantsPath = "/v1/operations/tenants/alpha/grants"
	const ledgerPath = "/v1/operations/tenants/alpha/ledger"
	const ratesPath = "/v1/operations/billing/rates?scope_type="
	var grants, nextGrants v1.ListTrafficGrantsResponse
	read(grantsPath+"?page_size=1", &grants)
	if len(grants.GetGrants()) != 1 || grants.GetNextPageToken() == "" {
		t.Fatalf("want one grant and continuation token, got %v", &grants)
	}
	read(grantsPath+"?page_token="+url.QueryEscape(grants.GetNextPageToken()), &nextGrants)
	if len(nextGrants.GetGrants()) != 1 || nextGrants.GetNextPageToken() != "" {
		t.Fatalf("want last grant, got %v", &nextGrants)
	}
	g1, g2 := grants.GetGrants()[0], nextGrants.GetGrants()[0]
	if g1.GetGrantedBytes() != 50 || g1.GetUsedBytes() != 50 || g1.GetOverdraftOffsetBytes() != 0 ||
		g2.GetGrantedBytes() != 25 || g2.GetUsedBytes() != 25 || g2.GetOverdraftOffsetBytes() != 25 ||
		grants.GetOverdraftBytes() != 15 || nextGrants.GetOverdraftBytes() != 15 {
		t.Fatalf("unexpected grants or overdraft: %v / %v", &grants, &nextGrants)
	}

	var global, tenant v1.ListBillingRatesResponse
	read(ratesPath+"global", &global)
	read(ratesPath+"tenant&scope_id=alpha", &tenant)
	if len(global.GetRates()) != 1 || len(tenant.GetRates()) != 1 ||
		global.GetRates()[0].GetUploadRatioMilli() != 1000 || tenant.GetRates()[0].GetUploadRatioMilli() != 2000 {
		t.Fatalf("unexpected rate versions: %v / %v", &global, &tenant)
	}
	r1, r2 := global.GetRates()[0].GetRateId(), tenant.GetRates()[0].GetRateId()
	if r1 == "" || r2 == "" || r1 == r2 {
		t.Fatalf("invalid rate IDs: %q, %q", r1, r2)
	}

	query := func(from, to time.Time, granularity string) string {
		return "/v1/operations/usage?" + url.Values{
			"tenant_id": {"alpha"}, "start_time": {from.Format(time.RFC3339)},
			"end_time": {to.Format(time.RFC3339)}, "granularity": {"USAGE_GRANULARITY_" + granularity},
		}.Encode()
	}
	const split = "&split_by_gateway=true&split_by_grant=true&split_by_rate=true"
	check := func(buckets []*v1.UsageBucket, at time.Time) {
		t.Helper()
		expected := []struct {
			grant, rate string
			raw, billed uint64
		}{{"", r2, 20, 40}, {g1.GetGrantId(), r1, 30, 30}, {g1.GetGrantId(), r2, 10, 20}}
		if len(buckets) != len(expected) {
			t.Fatalf("unexpected split buckets: %v", buckets)
		}
		byGrant := make(map[string]uint64)
		for i, want := range expected {
			b := buckets[i]
			amount := b.GetAmount()
			if b.GetGatewayId() != "gateway-a" || b.GetGrantId() != want.grant || b.GetRateId() != want.rate ||
				!b.GetStartTime().AsTime().Equal(at) || amount.GetInputBytes() != want.raw ||
				amount.GetOutputBytes() != 0 || amount.GetTotalBytes() != want.raw || amount.GetBillableBytes() != want.billed {
				t.Fatalf("unexpected bucket %d: %v", i, b)
			}
			byGrant[b.GetGrantId()] += amount.GetBillableBytes()
		}
		var offsets uint64
		for _, g := range []*v1.AuditedTrafficGrant{g1, g2} {
			if g.GetUsedBytes() != byGrant[g.GetGrantId()]+g.GetOverdraftOffsetBytes() {
				t.Fatalf("grant %q does not reconcile", g.GetGrantId())
			}
			offsets += g.GetOverdraftOffsetBytes()
		}
		if byGrant[""] != grants.GetOverdraftBytes()+offsets {
			t.Fatalf("debt bucket %d does not reconcile with balance %d and offsets %d", byGrant[""], grants.GetOverdraftBytes(), offsets)
		}
	}

	minute := query(start, start.Add(time.Minute), "MINUTE")
	var buckets []*v1.UsageBucket
	pageURL := minute + split + "&page_size=1"
	for i := range 3 {
		var page v1.QueryUsageResponse
		read(pageURL, &page)
		if len(page.GetBuckets()) != 1 || (page.GetNextPageToken() != "") != (i < 2) {
			t.Fatalf("unexpected split page %d: %v", i, &page)
		}
		buckets = append(buckets, page.GetBuckets()...)
		pageURL = minute + split + "&page_size=1&page_token=" + url.QueryEscape(page.GetNextPageToken())
	}
	check(buckets, start)
	var hour v1.QueryUsageResponse
	read(query(start, start.Add(time.Hour), "HOUR")+split, &hour)
	check(hour.GetBuckets(), start)

	for _, tc := range []struct {
		flags    string
		expected map[string]uint64
	}{{"&split_by_grant=true", map[string]uint64{"": 40, g1.GetGrantId(): 50}},
		{"&split_by_rate=true", map[string]uint64{r1: 30, r2: 60}}} {
		var result v1.QueryUsageResponse
		read(minute+tc.flags, &result)
		if len(result.GetBuckets()) != len(tc.expected) {
			t.Fatalf("unexpected single-dimension split: %v", &result)
		}
		for _, b := range result.GetBuckets() {
			if b.GetGrantId() != "" && b.GetRateId() != "" ||
				tc.expected[b.GetGrantId()+b.GetRateId()] != b.GetAmount().GetBillableBytes() {
				t.Fatalf("unexpected single-dimension bucket: %v", b)
			}
		}
	}

	midnight := start.Truncate(24 * time.Hour).Add(-8 * time.Hour)
	day := query(midnight, midnight.Add(24*time.Hour), "DAY") + split
	var dayBefore, dayAfter v1.QueryUsageResponse
	read(day, &dayBefore)
	if err := f.service.Archive(t.Context(), start.AddDate(0, 0, 61)); err != nil {
		t.Fatal(err)
	}
	read(day, &dayAfter)
	if !proto.Equal(&dayBefore, &dayAfter) {
		t.Fatalf("archive changed billing dimensions: %v -> %v", &dayBefore, &dayAfter)
	}
	check(dayAfter.GetBuckets(), midnight)

	var ledger, tail v1.ListBalanceLedgerResponse
	read(ledgerPath+"?page_size=1", &ledger)
	if len(ledger.GetEntries()) != 1 || ledger.GetNextPageToken() == "" {
		t.Fatalf("want first ledger entry and continuation token, got %v", &ledger)
	}
	read(ledgerPath+"?page_token="+url.QueryEscape(ledger.GetNextPageToken()), &tail)
	entries := append(ledger.GetEntries(), tail.GetEntries()...)
	if len(entries) != 3 || tail.GetNextPageToken() != "" {
		t.Fatalf("unexpected ledger entries: %v", entries)
	}
	if entries[0].GetKind() != "recharge" || entries[0].GetGrantId() != g1.GetGrantId() ||
		entries[0].GetDeltaBytes() != int64(g1.GetGrantedBytes()) || entries[0].GetExternalRef() != "first-grant" ||
		entries[1].GetKind() != "recharge" || entries[1].GetGrantId() != g2.GetGrantId() ||
		entries[1].GetDeltaBytes() != int64(g2.GetGrantedBytes()) || entries[1].GetExternalRef() != "offset-grant" ||
		entries[2].GetKind() != "overdraft_offset" || entries[2].GetGrantId() != g2.GetGrantId() ||
		entries[2].GetDeltaBytes() != -int64(g2.GetOverdraftOffsetBytes()) {
		t.Fatalf("ledger does not explain grant totals and offsets: %v", entries)
	}
	for _, token := range []string{f.alice, f.bob} {
		for _, path := range []string{grantsPath, ledgerPath, ratesPath + "global"} {
			testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", path, "", token), 403, "ACCESS_DENIED")
		}
	}
}
