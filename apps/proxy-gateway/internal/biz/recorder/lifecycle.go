package recorder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type LifecycleReporter interface {
	ReportLifecycle(context.Context, *v1.ReportLifecycleRequest) error
}

type LifecycleSpool interface {
	Boots() ([]string, error)
	ReadExit(boot string) ([]byte, error)
	ExitConfirmed(boot string) (bool, error)
	ConfirmExit(boot string) error
}

type Lifecycle struct {
	spool    LifecycleSpool
	reporter LifecycleReporter
}

func NewLifecycle(spool LifecycleSpool, reporter LifecycleReporter) *Lifecycle {
	return &Lifecycle{spool: spool, reporter: reporter}
}

// RunLifecycle reports the current child independently of file throughput and
// retries terminal markers left by any previous process.
func (r *Lifecycle) RunLifecycle(ctx context.Context, gateway, boot string, exited <-chan struct{}) {
	go r.runExitReports(ctx, gateway)
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
			if err := r.reporter.ReportLifecycle(ctx, &v1.ReportLifecycleRequest{GatewayId: gateway, BootId: boot}); err != nil && ctx.Err() == nil {
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

func (r *Lifecycle) runExitReports(ctx context.Context, gateway string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := r.reportExits(ctx, gateway); err != nil && ctx.Err() == nil {
			slog.Warn("gateway exit report failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Lifecycle) reportExits(ctx context.Context, gateway string) error {
	boots, err := r.spool.Boots()
	if err != nil {
		return err
	}
	var problems []error
	for _, boot := range boots {
		if err := r.ReportBootExit(ctx, gateway, boot); err != nil {
			problems = append(problems, fmt.Errorf("boot %s: %w", boot, err))
		}
	}
	return errors.Join(problems...)
}

// ReportBootExit sends an existing terminal marker. Its acknowledgement is
// durable, so a process restart can safely retry any unconfirmed marker.
func (r *Lifecycle) ReportBootExit(ctx context.Context, gateway, boot string) error {
	confirmed, err := r.spool.ExitConfirmed(boot)
	if err != nil || confirmed {
		return err
	}
	raw, err := r.spool.ReadExit(boot)
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
	if err := r.reporter.ReportLifecycle(ctx, &v1.ReportLifecycleRequest{
		GatewayId: gateway, BootId: boot, ExitedAt: timestamppb.New(exited),
	}); err != nil {
		return err
	}
	return r.spool.ConfirmExit(boot)
}
