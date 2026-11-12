package recorder

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"time"
)

type UnfinishedSession struct {
	SessionID, ClientID string
	LastRecordedAt      time.Time
}

func (c *Collector) reconcile(ctx context.Context, boot string, f *os.File, offset uint64, incompleteTail bool) error {
	exited, err := c.spool.ExitConfirmed(boot)
	if err != nil || !exited {
		if err == nil && incompleteTail {
			return errors.New("incomplete Recorder tail awaiting exit confirmation")
		}
		return err
	}
	info, err := f.Stat()
	if err != nil || !incompleteTail && uint64(info.Size()) != offset {
		return err
	}

	sessions, hasSessionRecords, err := c.unfinishedSessions(ctx, io.NewSectionReader(f, 0, int64(offset)))
	if err != nil {
		return err
	}
	pending := make([]UnfinishedSession, 0, len(sessions))
	for _, session := range sessions {
		pending = append(pending, session)
	}
	slices.SortFunc(pending, func(a, b UnfinishedSession) int { return strings.Compare(a.SessionID, b.SessionID) })
	for chunk := min(c.cfg.MaxRecords, 1000); len(pending) > 0; {
		n := min(chunk, len(pending))
		err := c.reporter.ReportUnfinishedSessions(ctx, c.cfg.GatewayID, boot, pending[:n])
		if errors.Is(err, ErrBatchTooLarge) {
			if n == 1 {
				return PermanentError{errors.New("control plane rejected an individual unfinished session as too large")}
			}
			chunk = n / 2
			continue
		}
		if err != nil {
			return err
		}
		pending = pending[n:]
	}
	if err := c.reporter.CompleteReplay(ctx, c.cfg.GatewayID, boot, uint32(len(sessions)), hasSessionRecords); err != nil {
		return err
	}
	if incompleteTail {
		// Retain the original file and offset for investigation.
		return nil
	}
	return c.spool.ConfirmReplay(boot)
}

func (c *Collector) unfinishedSessions(ctx context.Context, source io.Reader) (map[string]UnfinishedSession, bool, error) {
	reader := bufio.NewReaderSize(source, c.cfg.MaxBytes+2)
	sessions := make(map[string]UnfinishedSession)
	hasSessionRecords := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		line, err := reader.ReadSlice('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && len(line) == 0 {
				return sessions, hasSessionRecords, nil
			}
			return nil, false, err
		}
		var record struct {
			SessionID string
			ClientID  string
			Phase     string
			Time      time.Time
		}
		if json.Unmarshal(line, &record) != nil || !validReplayID(record.SessionID) {
			continue
		}
		if record.Phase == "final" {
			hasSessionRecords = true
			// GOST emits each session's final record after its last interim.
			delete(sessions, record.SessionID)
			continue
		}
		if record.Phase != "interim" ||
			!validReplayID(record.ClientID) || record.Time.IsZero() || record.Time.Year() < 1 || record.Time.Year() > 9999 {
			continue
		}
		hasSessionRecords = true
		previous := sessions[record.SessionID]
		if !record.Time.Before(previous.LastRecordedAt) {
			sessions[record.SessionID] = UnfinishedSession{
				SessionID: record.SessionID, ClientID: record.ClientID, LastRecordedAt: record.Time,
			}
		}
	}
}

func validReplayID(id string) bool {
	return id != "" && len(id) <= 128 && !strings.ContainsRune(id, 0)
}
