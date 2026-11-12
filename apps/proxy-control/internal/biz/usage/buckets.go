package usage

import (
	"cmp"
	"errors"
	"math"
	"math/bits"
	"slices"
	"time"
)

var errByteOverflow = errors.New("byte count exceeds storage range")

type bucketKey struct {
	TenantID    string
	UsageDate   string
	MinuteStart time.Time
	GrantID     int64
	RateID      int64
}

type Bucket struct {
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

func sortActivations(grants []*Grant) {
	slices.SortFunc(grants, func(a, b *Grant) int {
		if order := a.StartsAt.Compare(b.StartsAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func allocateRecord(tx Transaction, a *Account, r record, rate Rate, before string, now time.Time, buckets map[bucketKey]Bucket) error {
	input, output := *r.InputBytesDelta, *r.OutputBytesDelta
	total, err := billableBytes(input, output, rate.UploadRatioMilli, rate.DownloadRatioMilli)
	if err != nil {
		return err
	}

	key := bucketKey{TenantID: a.ID, UsageDate: r.Time.In(shanghai).Format(time.DateOnly), RateID: rate.ID}
	if key.UsageDate >= before {
		key.MinuteStart = r.Time.In(shanghai).Truncate(time.Minute).UTC()
	}
	var allocated, inputAllocated, outputAllocated int64
	for i := range a.Grants {
		g := &a.Grants[i]
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
		if a.Overdraft > math.MaxInt64-remaining {
			return errByteOverflow
		}
		a.Overdraft += remaining
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

func addBucket(buckets map[bucketKey]Bucket, key bucketKey, input, output, billed int64, count bool) error {
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

func writeBuckets(tx Transaction, gatewayID string, buckets map[bucketKey]Bucket, now time.Time) error {
	var minute, daily []Bucket
	for key, b := range buckets {
		b.TenantID, b.UsageDate, b.MinuteStart = key.TenantID, key.UsageDate, key.MinuteStart
		b.GrantID, b.RateID = key.GrantID, key.RateID
		b.GatewayID, b.Source, b.LastReceivedAt = gatewayID, "tunnel", now
		if key.MinuteStart.IsZero() {
			daily = append(daily, b)
		} else {
			minute = append(minute, b)
		}
	}
	return tx.WriteBuckets(minute, daily)
}
