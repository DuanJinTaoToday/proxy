package authmetrics

import (
	"expvar"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

var Counters = expvar.NewMap("proxy_gateway_auth")
var rejectionLogTimes sync.Map

func RecordLatency(path string, started time.Time) {
	elapsed := time.Since(started)
	var bucket string
	switch {
	case elapsed <= time.Millisecond:
		bucket = "le_1ms"
	case elapsed <= 5*time.Millisecond:
		bucket = "le_5ms"
	case elapsed <= 50*time.Millisecond:
		bucket = "le_50ms"
	case elapsed <= 200*time.Millisecond:
		bucket = "le_200ms"
	default:
		bucket = "gt_200ms"
	}
	Counters.Add(path+"_"+bucket, 1)
}

func RecordDecision(reason string) {
	Counters.Add("result_"+reason, 1)
	if reason == "OK" {
		return
	}
	// Authentication bursts must not produce one log write per rejection.
	second := time.Now().Unix()
	value, ok := rejectionLogTimes.Load(reason)
	if !ok {
		value, _ = rejectionLogTimes.LoadOrStore(reason, &atomic.Int64{})
	}
	last := value.(*atomic.Int64)
	previous := last.Load()
	if previous < second && last.CompareAndSwap(previous, second) {
		slog.Info("proxy authentication rejected", "reason", reason)
	}
}
