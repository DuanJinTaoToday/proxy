package authrelay

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/pkg/authmetrics"
)

const refreshBatchLimit = 100

func (r *Relay) Refresh(ctx context.Context) error {
	defer authmetrics.RecordLatency("refresh", time.Now())
	started := r.now()
	r.mu.Lock()
	epoch := r.epoch
	references := make([]string, 0, len(r.credentials))
	for username, entry := range r.credentials {
		if !started.Before(entry.synced.Add(stateLifetime)) {
			delete(r.credentials, username)
			continue
		}
		if entry.snapshot.GetActive() {
			references = append(references, entry.snapshot.GetReference())
		}
	}
	for tenant, balance := range r.quotas {
		if !validQuota(balance, started) {
			delete(r.quotas, tenant)
		}
	}
	r.mu.Unlock()
	slices.Sort(references)
	for batch := range slices.Chunk(references, refreshBatchLimit) {
		if err := r.refreshBatch(ctx, batch, epoch); err != nil {
			if r.authInterrupted != nil {
				r.authInterrupted()
			}
			authmetrics.Counters.Add("refresh_failed", 1)
			return err
		}
	}
	return nil
}

func (r *Relay) refreshBatch(ctx context.Context, references []string, epoch uint64) error {
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	started := r.now()
	generation, renewQuota := r.quotaRenewal()
	snapshots, err := r.control.Refresh(ctx, references)
	if err != nil {
		r.invalidateOnDenial(err, epoch)
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(snapshots) != len(references) {
		return errors.New("incomplete refresh")
	}
	requested := make(map[string]bool, len(references))
	for _, ref := range references {
		requested[ref] = true
	}
	for _, snapshot := range snapshots {
		if !validSnapshot(snapshot) || !requested[snapshot.GetReference()] {
			return errors.New("invalid refresh state")
		}
		delete(requested, snapshot.GetReference())
	}
	r.apply(snapshots, started, epoch, generation, renewQuota)
	return nil
}

func (r *Relay) RunRefresh(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Refresh(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("proxy auth refresh failed")
			}
		}
	}
}
