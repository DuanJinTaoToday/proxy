package recorder

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
	"unicode/utf8"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

const (
	fileID = "sessions"
)

type Reporter interface {
	Submit(context.Context, *v1.SubmitBatchRequest) error
}

type Spool interface {
	Boots() ([]string, error)
	SessionSize(boot string) (int64, error)
	OpenSession(boot string) (*os.File, error)
	Offset(boot string) (uint64, error)
	SaveOffset(boot string, offset uint64) error
	StatExit(boot string) error
}

type Config struct {
	GatewayID  string
	MaxRecords int
	MaxBytes   int
	Interval   time.Duration
}

type retry struct {
	at        time.Time
	delay     time.Duration
	permanent bool
}

type Collector struct {
	cfg            Config
	reporter       Reporter
	spool          Spool
	retries        map[string]retry
	pendingSince   map[string]time.Time
	lastBacklogLog time.Time
}

func New(cfg Config, spool Spool, reporter Reporter) *Collector {
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = 1000
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 1 << 20
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Second
	}
	return &Collector{cfg: cfg, spool: spool, reporter: reporter, retries: make(map[string]retry), pendingSince: make(map[string]time.Time)}
}

func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()
	for {
		if err := c.Scan(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("Recorder scan", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Scan advances each boot by at most one accepted batch. A failed boot does
// not prevent other boots from submitting their own contiguous records.
func (c *Collector) Scan(ctx context.Context) error {
	boots, err := c.spool.Boots()
	if err != nil {
		return err
	}
	var errs []error
	var pendingBytes uint64
	var oldest time.Time
	now := time.Now()
	for _, boot := range boots {
		if err := ctx.Err(); err != nil {
			return err
		}
		if size, statErr := c.spool.SessionSize(boot); statErr == nil {
			if confirmed, offsetErr := c.spool.Offset(boot); offsetErr == nil && uint64(size) > confirmed {
				pendingBytes += uint64(size) - confirmed
				since := c.pendingSince[boot]
				if since.IsZero() {
					since = now
					c.pendingSince[boot] = since
				}
				if oldest.IsZero() || since.Before(oldest) {
					oldest = since
				}
			} else {
				delete(c.pendingSince, boot)
			}
		}
		state := c.retries[boot]
		if state.permanent || time.Now().Before(state.at) {
			continue
		}
		err := c.scanBoot(ctx, boot)
		if err == nil {
			delete(c.retries, boot)
			continue
		}
		var stopped PermanentError
		if errors.As(err, &stopped) {
			state.permanent = true
		} else {
			state.delay = min(max(time.Second, 2*state.delay), 30*time.Second)
			state.at = time.Now().Add(state.delay)
		}
		c.retries[boot] = state
		errs = append(errs, fmt.Errorf("boot %s: %w", boot, err))
	}
	if !oldest.IsZero() && now.Sub(oldest) >= 5*time.Second && now.Sub(c.lastBacklogLog) >= 5*time.Second {
		slog.Warn("Recorder pending bytes", "bytes", pendingBytes, "oldest_observed_wait", now.Sub(oldest))
		c.lastBacklogLog = now
	}
	return errors.Join(errs...)
}

func (c *Collector) scanBoot(ctx context.Context, boot string) error {
	f, err := c.spool.OpenSession(boot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	offset, err := c.spool.Offset(boot)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || offset > uint64(info.Size()) {
		return PermanentError{errors.New("recorder file is not regular or is shorter than its confirmed offset")}
	}
	if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
		return err
	}
	batch := &v1.SubmitBatchRequest{GatewayId: c.cfg.GatewayID, BootId: boot}
	empty, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	// encoding/json is shared by the HTTP client. This charges the array key,
	// delimiters, and the exact encoded size of each RecorderLine.
	size := len(empty) + len(`,"records":[]`)
	reader := bufio.NewReaderSize(f, c.cfg.MaxBytes+2)
	var readErr error
	for len(batch.Records) < c.cfg.MaxRecords {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			switch {
			case errors.Is(err, bufio.ErrBufferFull):
				readErr = PermanentError{fmt.Errorf("oversized Recorder line at offset %d", offset)}
			case errors.Is(err, io.EOF):
				if len(line) != 0 {
					exitErr := c.spool.StatExit(boot)
					if exitErr == nil {
						readErr = PermanentError{fmt.Errorf("closed boot has an incomplete line at offset %d", offset)}
					} else if !errors.Is(exitErr, os.ErrNotExist) {
						readErr = exitErr
					}
				}
			default:
				readErr = err
			}
			break
		}
		content := bytes.TrimSuffix(line[:len(line)-1], []byte{'\r'})
		if !utf8.Valid(content) {
			readErr = PermanentError{fmt.Errorf("invalid UTF-8 at offset %d", offset)}
			break
		}
		record := &v1.RecorderLine{FileId: fileID, FileOffset: offset, EndOffset: offset + uint64(len(line)), Line: string(content)}
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		size += len(encoded)
		if len(batch.Records) != 0 {
			size++
		}
		if size > c.cfg.MaxBytes {
			if len(batch.Records) == 0 {
				readErr = PermanentError{fmt.Errorf("recorder line exceeds batch byte limit at offset %d", offset)}
			}
			break
		}
		batch.Records = append(batch.Records, record)
		offset = record.EndOffset
	}
	if len(batch.Records) != 0 {
		count := len(batch.Records)
		if err := c.submit(ctx, boot, batch); err != nil {
			return err
		}
		if len(batch.Records) < count {
			return nil
		}
	}
	return readErr
}

func (c *Collector) submit(ctx context.Context, boot string, batch *v1.SubmitBatchRequest) error {
	for {
		err := c.reporter.Submit(ctx, batch)
		if errors.Is(err, ErrBatchTooLarge) && len(batch.Records) > 1 {
			batch.Records = batch.Records[:len(batch.Records)/2]
			continue
		}
		if errors.Is(err, ErrBatchTooLarge) {
			return PermanentError{errors.New("control plane rejected an individual line as too large")}
		}
		if err != nil {
			return err
		}
		return c.spool.SaveOffset(boot, batch.Records[len(batch.Records)-1].EndOffset)
	}
}

var ErrBatchTooLarge = errors.New("recorder batch too large")

type PermanentError struct{ Err error }

func (e PermanentError) Error() string { return e.Err.Error() }
