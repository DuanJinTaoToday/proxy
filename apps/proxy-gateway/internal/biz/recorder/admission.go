package recorder

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"
)

const reportingGrace = 5 * time.Second

type pendingFile struct {
	offset uint64
	since  time.Time
}

type admissionState struct {
	mu              sync.Mutex
	blocked         bool
	generation      uint64
	deadline        time.Time
	observedAt      time.Time
	authInterrupted bool

	// Only the scan loop reads or changes these fields.
	pending     map[string]pendingFile
	targets     map[string]uint64
	failed      bool
	submissions uint64
}

// AuthInterrupted requires a new spool observation before quota can renew.
func (c *Collector) AuthInterrupted() {
	a := &c.admission
	a.mu.Lock()
	defer a.mu.Unlock()
	a.blocked = true
	a.generation++
	a.authInterrupted = true
}

// QuotaRenewal identifies the reporting state in which a quota request starts
// or completes. Both observations must be eligible and in the same generation.
func (c *Collector) QuotaRenewal() (uint64, bool) {
	a := &c.admission
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expire(c.now())
	return a.generation, !a.blocked
}

func (a *admissionState) expire(now time.Time) {
	// A scan stalled in reporting cannot prove that no new file or complete
	// record has appeared since the last observation.
	stale := !now.Before(a.observedAt.Add(reportingGrace))
	overdue := !a.deadline.IsZero() && !now.Before(a.deadline)
	if (stale || overdue) && !a.blocked {
		a.blocked = true
		a.generation++
	}
}

func (a *admissionState) block() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.blocked {
		a.blocked = true
		a.generation++
	}
}

func (a *admissionState) failSubmission() {
	a.block()
	a.failed = true
	a.targets = nil
}

func (c *Collector) observeAdmission(boots []string) error {
	a := &c.admission
	now := c.now()
	next := make(map[string]pendingFile)
	var deadline time.Time
	for _, boot := range boots {
		confirmed, end, err := c.completeOffsets(boot)
		if err != nil {
			return err
		}
		if end <= confirmed {
			continue
		}
		pending, exists := a.pending[boot]
		if !exists || pending.offset != confirmed {
			pending = pendingFile{offset: confirmed, since: now}
			if recordedAt := c.pendingRecordTime(boot, confirmed); !recordedAt.IsZero() && recordedAt.Before(now) {
				pending.since = recordedAt
			}
		}
		next[boot] = pending
		due := pending.since.Add(reportingGrace)
		if deadline.IsZero() || due.Before(deadline) {
			deadline = due
		}
	}
	a.pending = next
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expire(c.now())
	a.deadline = deadline
	a.observedAt = now
	a.expire(c.now())
	return nil
}

func (c *Collector) finishAdmission(submissions uint64) error {
	a := &c.admission
	a.mu.Lock()
	if a.authInterrupted {
		a.targets = nil
		a.authInterrupted = false
	}
	a.mu.Unlock()
	boots, err := c.spool.Boots()
	if err != nil {
		return err
	}
	_, allowed := c.QuotaRenewal()
	if allowed {
		return c.observeAdmission(boots)
	}
	if a.failed && a.submissions == submissions {
		return nil
	}
	if a.targets == nil {
		a.targets = make(map[string]uint64)
		for _, boot := range boots {
			confirmed, end, err := c.completeOffsets(boot)
			if err != nil {
				a.targets = nil
				return err
			}
			if end > confirmed {
				a.targets[boot] = end
			}
		}
		a.failed = false
	}
	for boot, target := range a.targets {
		confirmed, err := c.spool.Offset(boot)
		if err != nil {
			return err
		}
		if confirmed < target {
			return nil
		}
	}
	if err := c.observeAdmission(boots); err != nil {
		return err
	}
	a.targets = nil
	a.mu.Lock()
	if !a.authInterrupted {
		a.blocked = false
		a.generation++
		a.expire(c.now())
	}
	a.mu.Unlock()
	return nil
}

// Read backwards from the sampled file size. Bytes after the last LF are a
// partial tail and never become a recovery target.
func (c *Collector) completeOffsets(boot string) (confirmed, end uint64, err error) {
	size, err := c.spool.SessionSize(boot)
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	confirmed, err = c.spool.Offset(boot)
	if err != nil {
		return 0, 0, err
	}
	if size < 0 || confirmed > uint64(size) {
		return 0, 0, fmt.Errorf("confirmation offset exceeds Recorder file size for boot %s", boot)
	}
	if confirmed == uint64(size) {
		return confirmed, confirmed, nil
	}
	f, err := c.spool.OpenSession(boot)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return 0, 0, err
	}
	if !info.Mode().IsRegular() || confirmed > uint64(info.Size()) {
		return 0, 0, fmt.Errorf("invalid Recorder file or confirmation offset for boot %s", boot)
	}
	var chunk [4096]byte
	lower := max(int64(confirmed), size-int64(c.cfg.MaxBytes)-2)
	for pos := size; pos > lower; {
		start := max(lower, pos-int64(len(chunk)))
		data := chunk[:pos-start]
		if _, err := f.ReadAt(data, start); err != nil {
			return 0, 0, err
		}
		if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
			return confirmed, uint64(start) + uint64(i) + 1, nil
		}
		pos = start
	}
	if lower > int64(confirmed) {
		return 0, 0, fmt.Errorf("oversized Recorder tail for boot %s", boot)
	}
	return confirmed, confirmed, nil
}
