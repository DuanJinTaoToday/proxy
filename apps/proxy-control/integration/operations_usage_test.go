package integration

import (
	"crypto/sha256"
	"net/url"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

func TestOperationsUsageBuckets(t *testing.T) {
	f := setupUsage(t)
	operator := testfixture.Login(t, f.http, "operator")
	const gatewayBToken = otherAuthToken
	hash := sha256.Sum256([]byte(gatewayBToken))
	execSQL(t, f.db, "INSERT INTO gateway_tokens (gateway_id, scope, token_hash) VALUES ('gateway-b', 'report', ?)", hash[:])

	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 2, 15, 0, 0, time.UTC)
	a := recorder(0, "client-a", start.Format(time.RFC3339), 10, 20, 10, 20)
	excluded := recorder(a.GetEndOffset(), "client-a", start.Add(-time.Minute).Format(time.RFC3339), 100, 200, 100, 200)
	b := recorder(excluded.GetEndOffset(), "client-a", start.Add(time.Minute).Format(time.RFC3339), 1, 2, 1, 2)
	otherTenant := recorder(b.GetEndOffset(), "client-b", start.Format(time.RFC3339), 99, 99, 99, 99)
	end := start.Add(time.Hour)
	atEnd := recorder(otherTenant.GetEndOffset(), "client-a", end.Format(time.RFC3339), 88, 88, 88, 88)
	lines := []*v1.RecorderLine{a, excluded, b, otherTenant, atEnd}
	offset := atEnd.GetEndOffset()
	for i := range 101 {
		record := recorder(offset, "client-a", end.Add(time.Duration(i+1)*time.Minute).Format(time.RFC3339), 1, 2, 1, 2)
		lines = append(lines, record)
		offset = record.GetEndOffset()
	}
	if got := submit(t, f, "gateway-a", reportToken, lines...).GetBillable(); got != uint32(len(lines)) {
		t.Fatalf("gateway-a ingested %d records, want %d", got, len(lines))
	}
	c := recorder(0, "client-a", start.Format(time.RFC3339), 4, 5, 4, 5)
	if got := submit(t, f, "gateway-b", gatewayBToken, c).GetBillable(); got != 1 {
		t.Fatalf("gateway-b ingested %d records, want 1", got)
	}

	query := func(from, to time.Time, granularity string) string {
		return "/v1/operations/usage?" + url.Values{
			"tenant_id": {"alpha"}, "start_time": {from.Format(time.RFC3339)},
			"end_time": {to.Format(time.RFC3339)}, "granularity": {"USAGE_GRANULARITY_" + granularity},
		}.Encode()
	}
	read := func(path string) *v1.QueryUsageResponse {
		t.Helper()
		var result v1.QueryUsageResponse
		readProto(t, testfixture.Request(f.http, "GET", path, "", operator), &result)
		return &result
	}
	assertBucket := func(result *v1.QueryUsageResponse, index int, at time.Time, gateway string, in, out uint64) {
		t.Helper()
		if len(result.GetBuckets()) <= index {
			t.Fatalf("missing bucket %d in %v", index, result)
		}
		got := result.GetBuckets()[index]
		if !got.GetStartTime().AsTime().Equal(at) || got.GetGatewayId() != gateway ||
			got.GetAmount().GetInputBytes() != in || got.GetAmount().GetOutputBytes() != out ||
			got.GetAmount().GetTotalBytes() != in+out || got.GetAmount().GetBillableBytes() != in+out {
			t.Fatalf("bucket %d = %v, want %s %q %d/%d", index, got, at, gateway, in, out)
		}
	}
	assertBadRequest := func(path, reason string) {
		t.Helper()
		testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", path, "", operator), 400, reason)
	}

	minute := query(start, end, "MINUTE")
	minutes := read(minute)
	if len(minutes.GetBuckets()) != 2 {
		t.Fatalf("want two minute buckets, got %v", minutes)
	}
	assertBucket(minutes, 0, start, "", 14, 25)
	assertBucket(minutes, 1, start.Add(time.Minute), "", 1, 2)

	hours := read(query(start, end, "HOUR"))
	if len(hours.GetBuckets()) != 1 {
		t.Fatalf("want one hour bucket, got %v", hours)
	}
	assertBucket(hours, 0, start.Truncate(time.Hour), "", 15, 27)
	filtered := read(query(start, end, "HOUR") + "&gateway_id=gateway-b")
	if len(filtered.GetBuckets()) != 1 {
		t.Fatalf("want one filtered hour bucket, got %v", filtered)
	}
	assertBucket(filtered, 0, start.Truncate(time.Hour), "", 4, 5)
	midnight := start.Truncate(24 * time.Hour).Add(-8 * time.Hour)
	day := query(midnight, midnight.Add(24*time.Hour), "DAY")
	assertBucket(read(day+"&gateway_id=gateway-b&split_by_gateway=true"), 0, midnight, "gateway-b", 4, 5)
	partialDay := query(midnight, start, "DAY") + "&gateway_id=gateway-b"
	assertBucket(read(partialDay), 0, midnight, "", 4, 5)
	afterMidnight := query(midnight.Add(time.Minute), midnight.Add(24*time.Hour), "DAY") + "&gateway_id=gateway-b"
	if got := read(afterMidnight); len(got.GetBuckets()) != 0 {
		t.Fatalf("daily bucket started before the window: %v", got)
	}

	split := minute + "&split_by_gateway=true&page_size=1"
	page := read(split)
	assertBucket(page, 0, start, "gateway-a", 10, 20)
	for _, expected := range []struct {
		at      time.Time
		gateway string
		in, out uint64
	}{{start, "gateway-b", 4, 5}, {start.Add(time.Minute), "gateway-a", 1, 2}} {
		if len(page.GetBuckets()) != 1 || page.GetNextPageToken() == "" {
			t.Fatalf("want one bucket and continuation token, got %v", page)
		}
		token := "&page_token=" + url.QueryEscape(page.GetNextPageToken())
		assertBadRequest(split+"&gateway_id=gateway-b"+token, "INVALID_PAGE_TOKEN")
		page = read(split + token)
		assertBucket(page, 0, expected.at, expected.gateway, expected.in, expected.out)
	}
	if len(page.GetBuckets()) != 1 || page.GetNextPageToken() != "" {
		t.Fatalf("want one bucket and no continuation token, got %v", page)
	}

	defaultPage := query(end.Add(time.Minute), end.Add(102*time.Minute), "MINUTE")
	first := read(defaultPage)
	if len(first.GetBuckets()) != 100 || first.GetNextPageToken() == "" {
		t.Fatalf("default page returned %d buckets with token=%t", len(first.GetBuckets()), first.GetNextPageToken() != "")
	}
	last := read(defaultPage + "&page_size=500&page_token=" + url.QueryEscape(first.GetNextPageToken()))
	if len(last.GetBuckets()) != 1 || last.GetNextPageToken() != "" {
		t.Fatalf("want one remaining bucket, got %v", last)
	}
	assertBucket(last, 0, end.Add(101*time.Minute), "", 1, 2)

	assertBadRequest(minute+"&page_size=501", "INVALID_PAGE_SIZE")
	assertBadRequest(minute+"&page_token=invalid!", "INVALID_PAGE_TOKEN")
	assertBadRequest(query(start, start.Add(24*time.Hour+time.Second), "MINUTE"), "INVALID_TIME_RANGE")
	assertBadRequest(query(start, start.Add(31*24*time.Hour+time.Second), "HOUR"), "INVALID_TIME_RANGE")
	assertBadRequest(query(start.AddDate(0, 0, -61), end.AddDate(0, 0, -61), "HOUR"), "INVALID_TIME_RANGE")

	if err := f.service.Archive(t.Context(), start.AddDate(0, 0, 61)); err != nil {
		t.Fatal(err)
	}
	archived := read(day + "&gateway_id=gateway-b&split_by_gateway=true")
	if len(archived.GetBuckets()) != 1 {
		t.Fatalf("want one archived gateway bucket, got %v", archived)
	}
	assertBucket(archived, 0, midnight, "gateway-b", 4, 5)
	assertBucket(read(partialDay), 0, midnight, "", 4, 5)
	if got := read(afterMidnight); len(got.GetBuckets()) != 0 {
		t.Fatalf("archived daily bucket started before the window: %v", got)
	}
}
