package recorder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const exitAckFile = "exit.confirmed"

func (r *HTTPReporter) ReportLifecycle(ctx context.Context, req *v1.ReportLifecycleRequest) error {
	body, err := protojson.Marshal(req)
	if err != nil {
		return err
	}
	data, err := r.post(ctx, "/v1/reporting/lifecycle", json.RawMessage(body))
	if err != nil {
		return err
	}
	var ack v1.ReportLifecycleResponse
	return protojson.Unmarshal(data, &ack)
}

// RunLifecycle reports the current child independently of file throughput and
// retries terminal markers left by any previous process.
func (r *HTTPReporter) RunLifecycle(ctx context.Context, spool, gateway, boot string, exited <-chan struct{}) {
	go r.runExitReports(ctx, spool, gateway)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-exited:
			boot = ""
			exited = nil
		default:
		}
		if boot != "" {
			if err := r.ReportLifecycle(ctx, &v1.ReportLifecycleRequest{GatewayId: gateway, BootId: boot}); err != nil && ctx.Err() == nil {
				slog.Warn("gateway heartbeat failed", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-exited:
			boot = ""
			exited = nil
		}
	}
}

func (r *HTTPReporter) runExitReports(ctx context.Context, spool, gateway string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := r.reportExits(ctx, spool, gateway); err != nil && ctx.Err() == nil {
			slog.Warn("gateway exit report failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *HTTPReporter) reportExits(ctx context.Context, spool, gateway string) error {
	entries, err := os.ReadDir(spool)
	if err != nil {
		return err
	}
	var problems []error
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}
		if err := r.ReportBootExit(ctx, spool, gateway, entry.Name()); err != nil {
			problems = append(problems, fmt.Errorf("boot %s: %w", entry.Name(), err))
		}
	}
	return errors.Join(problems...)
}

// ReportBootExit sends an existing terminal marker. Its acknowledgement is
// durable, so a process restart can safely retry any unconfirmed marker.
func (r *HTTPReporter) ReportBootExit(ctx context.Context, spool, gateway, boot string) error {
	dir := filepath.Join(spool, boot)
	if _, err := os.Stat(filepath.Join(dir, exitAckFile)); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(dir, ExitFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	exited, err := time.Parse(time.RFC3339Nano, string(raw))
	if err != nil {
		return errors.New("invalid GOST exit marker")
	}
	if err := r.ReportLifecycle(ctx, &v1.ReportLifecycleRequest{
		GatewayId: gateway, BootId: boot, ExitedAt: timestamppb.New(exited),
	}); err != nil {
		return err
	}
	return WriteDurable(dir, exitAckFile, "confirmed\n")
}
