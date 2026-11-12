package repo

import (
	"context"
	"log/slog"
	"time"
)

func usedPercent(available, total uint64) uint64 {
	if total == 0 || available >= total {
		return 0
	}
	return uint64(100 * float64(total-available) / float64(total))
}

func MonitorDisk(ctx context.Context, spool string) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		used, err := diskUsage(spool)
		switch {
		case err != nil:
			slog.Warn("Recorder disk usage unavailable", "error", err)
		case used >= 90:
			slog.Error("Recorder disk usage critical", "percent", used)
		case used >= 80:
			slog.Warn("Recorder disk usage high", "percent", used)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
