package usage

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errByteOverflow = errors.New("byte count exceeds storage range")

type bucketKey struct {
	TenantID    string
	UsageDate   string
	MinuteStart time.Time
	GrantID     int64
	RateID      int64
}

type bucket struct {
	TenantID       string
	GatewayID      string
	UsageDate      string
	MinuteStart    time.Time
	GrantID        int64
	RateID         int64
	Source         string
	InputBytes     int64
	OutputBytes    int64
	BillableBytes  int64
	RecordCount    int64
	LastReceivedAt time.Time
}

func sortActivations(grants []*grant) {
	slices.SortFunc(grants, func(a, b *grant) int {
		if order := a.StartsAt.Compare(b.StartsAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func allocateRecord(tx *gorm.DB, a *account, r record, rate model.BillingRate, before string, now time.Time, buckets map[bucketKey]bucket) error {
	input, output := *r.InputBytesDelta, *r.OutputBytesDelta
	total, err := billableBytes(input, output, rate.UploadRatioMilli, rate.DownloadRatioMilli)
	if err != nil {
		return err
	}

	key := bucketKey{TenantID: a.id, UsageDate: r.Time.In(shanghai).Format(time.DateOnly), RateID: rate.ID}
	if key.UsageDate >= before {
		key.MinuteStart = r.Time.In(shanghai).Truncate(time.Minute).UTC()
	}
	var allocated, inputAllocated, outputAllocated int64
	for i := range a.grants {
		g := &a.grants[i]
		if allocated == total {
			break
		}
		debit := min(total-allocated, g.GrantedBytes-g.UsedBytes)
		if !g.activeAt(r.Time) || g.ActivatedAt == nil || debit == 0 {
			continue
		}
		g.UsedBytes += debit
		g.dirty = true
		if err := a.correctExpiry(tx, g, debit, now); err != nil {
			return err
		}
		next := allocated + debit
		nextInput, nextOutput := prorate(input, next, total), prorate(output, next, total)
		key.GrantID = g.ID
		if err := addBucket(buckets, key, nextInput-inputAllocated, nextOutput-outputAllocated, debit, allocated == 0); err != nil {
			return err
		}
		allocated, inputAllocated, outputAllocated = next, nextInput, nextOutput
	}
	if allocated < total || total == 0 {
		remaining := total - allocated
		if a.overdraft > math.MaxInt64-remaining {
			return errByteOverflow
		}
		a.overdraft += remaining
		key.GrantID = 0
		return addBucket(buckets, key, input-inputAllocated, output-outputAllocated, remaining, allocated == 0)
	}
	return nil
}

func prorate(raw, allocated, total int64) int64 {
	hi, lo := bits.Mul64(uint64(raw), uint64(allocated))
	value, _ := bits.Div64(hi, lo, uint64(total))
	return int64(value)
}

func addBucket(buckets map[bucketKey]bucket, key bucketKey, input, output, billed int64, count bool) error {
	b := buckets[key]
	if b.InputBytes > math.MaxInt64-input || b.OutputBytes > math.MaxInt64-output || b.BillableBytes > math.MaxInt64-billed {
		return errByteOverflow
	}
	b.InputBytes += input
	b.OutputBytes += output
	b.BillableBytes += billed
	if count {
		b.RecordCount++
	}
	buckets[key] = b
	return nil
}

func writeBuckets(tx *gorm.DB, gatewayID string, buckets map[bucketKey]bucket, now time.Time) error {
	var minute, daily []bucket
	dates := make(map[string]bool)
	for key, b := range buckets {
		b.TenantID, b.UsageDate, b.MinuteStart = key.TenantID, key.UsageDate, key.MinuteStart
		b.GrantID, b.RateID = key.GrantID, key.RateID
		b.GatewayID, b.Source, b.LastReceivedAt = gatewayID, "tunnel", now
		if key.MinuteStart.IsZero() {
			daily = append(daily, b)
		} else {
			minute = append(minute, b)
			dates[key.UsageDate] = true
		}
	}
	for date := range dates {
		if err := ensurePartition(tx, date); err != nil {
			return err
		}
	}
	if len(minute) != 0 {
		if err := upsertBuckets(tx, "usage_minutely", minute, true); err != nil {
			return err
		}
	}
	if len(daily) != 0 {
		return upsertBuckets(tx, "usage_daily", daily, false)
	}
	return nil
}

func upsertBuckets(tx *gorm.DB, table string, rows []bucket, minute bool) error {
	keys := []clause.Column{{Name: "tenant_id"}, {Name: "gateway_id"}, {Name: "usage_date"}, {Name: "grant_id"}, {Name: "rate_id"}, {Name: "source"}}
	query := tx.Table(table)
	if minute {
		keys = append(keys, clause.Column{Name: "minute_start"})
	} else {
		query = query.Omit("MinuteStart")
	}
	updates := make(map[string]any)
	for _, col := range []string{"input_bytes", "output_bytes", "billable_bytes", "record_count"} {
		updates[col] = gorm.Expr(fmt.Sprintf("%s.%s + EXCLUDED.%s", table, col, col))
	}
	updates["last_received_at"] = gorm.Expr("GREATEST(" + table + ".last_received_at, EXCLUDED.last_received_at)")
	return query.Clauses(clause.OnConflict{Columns: keys, DoUpdates: clause.Assignments(updates)}).CreateInBatches(rows, 500).Error
}
