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
	"path/filepath"
	"strconv"
	"time"
	"unicode/utf8"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

const (
	SessionFile = "sessions.jsonl"
	ExitFile    = "exited_at"
	cursorFile  = "confirmed.offset"
	fileID      = "sessions"
)

type Reporter interface {
	Submit(context.Context, *v1.SubmitBatchRequest) error
}

type Config struct {
	Spool      string
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
	retries        map[string]retry
	pendingSince   map[string]time.Time
	lastBacklogLog time.Time
}

func New(cfg Config, reporter Reporter) *Collector {
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = 1000
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 1 << 20
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Second
	}
	return &Collector{cfg: cfg, reporter: reporter, retries: make(map[string]retry), pendingSince: make(map[string]time.Time)}
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
	entries, err := os.ReadDir(c.cfg.Spool)
	if err != nil {
		return err
	}
	var errs []error
	var pendingBytes uint64
	var oldest time.Time
	now := time.Now()
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}
		boot := entry.Name()
		dir := filepath.Join(c.cfg.Spool, boot)
		if info, statErr := os.Stat(filepath.Join(dir, SessionFile)); statErr == nil {
			if confirmed, offsetErr := loadOffset(dir); offsetErr == nil && uint64(info.Size()) > confirmed {
				pendingBytes += uint64(info.Size()) - confirmed
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
		var stopped permanentError
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
	dir := filepath.Join(c.cfg.Spool, boot)
	f, err := os.Open(filepath.Join(dir, SessionFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	offset, err := loadOffset(dir)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || offset > uint64(info.Size()) {
		return permanentError{errors.New("recorder file is not regular or is shorter than its confirmed offset")}
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
				readErr = permanentError{fmt.Errorf("oversized Recorder line at offset %d", offset)}
			case errors.Is(err, io.EOF):
				if len(line) != 0 {
					_, exitErr := os.Stat(filepath.Join(dir, ExitFile))
					if exitErr == nil {
						readErr = permanentError{fmt.Errorf("closed boot has an incomplete line at offset %d", offset)}
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
			readErr = permanentError{fmt.Errorf("invalid UTF-8 at offset %d", offset)}
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
				readErr = permanentError{fmt.Errorf("recorder line exceeds batch byte limit at offset %d", offset)}
			}
			break
		}
		batch.Records = append(batch.Records, record)
		offset = record.EndOffset
	}
	if len(batch.Records) != 0 {
		count := len(batch.Records)
		if err := c.submit(ctx, dir, batch); err != nil {
			return err
		}
		if len(batch.Records) < count {
			return nil
		}
	}
	return readErr
}

func (c *Collector) submit(ctx context.Context, dir string, batch *v1.SubmitBatchRequest) error {
	for {
		err := c.reporter.Submit(ctx, batch)
		if errors.Is(err, ErrBatchTooLarge) && len(batch.Records) > 1 {
			batch.Records = batch.Records[:len(batch.Records)/2]
			continue
		}
		if errors.Is(err, ErrBatchTooLarge) {
			return permanentError{errors.New("control plane rejected an individual line as too large")}
		}
		if err != nil {
			return err
		}
		return WriteDurable(dir, cursorFile, strconv.FormatUint(batch.Records[len(batch.Records)-1].EndOffset, 10)+"\n")
	}
}

func loadOffset(dir string) (uint64, error) {
	raw, err := os.ReadFile(filepath.Join(dir, cursorFile))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	offset, err := strconv.ParseUint(string(bytes.TrimSpace(raw)), 10, 64)
	if err != nil {
		return 0, permanentError{errors.New("invalid persisted Recorder offset")}
	}
	return offset, nil
}

type permanentError struct{ error }
