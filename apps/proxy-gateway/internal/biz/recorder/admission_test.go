package recorder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

type admissionSpool struct {
	Spool
	root    string
	offsets map[string]uint64
}

func (s admissionSpool) Boots() ([]string, error) { return []string{"a", "b"}, nil }
func (s admissionSpool) OpenSession(boot string) (*os.File, error) {
	return os.Open(filepath.Join(s.root, boot))
}
func (s admissionSpool) SessionSize(boot string) (int64, error) {
	info, err := os.Stat(filepath.Join(s.root, boot))
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
func (s admissionSpool) Offset(boot string) (uint64, error) { return s.offsets[boot], nil }
func (s admissionSpool) SaveOffset(boot string, offset uint64) error {
	s.offsets[boot] = offset
	return nil
}
func (s admissionSpool) ReplayCompleted(string) (bool, error) { return false, nil }
func (s admissionSpool) ExitConfirmed(string) (bool, error)   { return false, nil }
func (s admissionSpool) StatExit(string) error                { return os.ErrNotExist }

type admissionReporter struct {
	Reporter
	fail bool
}

func (r *admissionReporter) Submit(context.Context, *v1.SubmitBatchRequest) error {
	if r.fail {
		return errors.New("lost report response")
	}
	return nil
}

func TestQuotaRenewalWaitsForFixedOffsetsAcrossFiles(t *testing.T) {
	spool := admissionSpool{root: t.TempDir(), offsets: make(map[string]uint64)}
	reporter := &admissionReporter{}
	collector := New(Config{MaxRecords: 1}, spool, reporter)
	now := time.Now()
	collector.now = func() time.Time { return now }
	write := func(boot, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(spool.root, boot), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	scan := func(wantError bool) {
		t.Helper()
		if err := collector.Scan(t.Context()); (err != nil) != wantError {
			t.Fatalf("scan error = %v", err)
		}
	}
	allowed := func(want bool) {
		t.Helper()
		if _, got := collector.QuotaRenewal(); got != want {
			t.Fatalf("quota renewal = %t, want %t", got, want)
		}
	}
	write("a", "")
	write("b", "partial")
	allowed(false)
	scan(false)
	allowed(true)
	write("a", "{}\n{}\n")
	write("b", "{}\n{}\n{}\npartial")
	reporter.fail = true
	scan(true)
	allowed(false)
	reporter.fail = false
	now = now.Add(time.Second)
	scan(false)
	allowed(false)
	// Append beyond a's fixed recovery point while b still needs two scans.
	write("a", "{}\n{}\n{}\n{}\n{}\n")
	scan(false)
	allowed(false)
	scan(false)
	allowed(true)
	if spool.offsets["a"] != 9 || spool.offsets["b"] != 9 {
		t.Fatalf("unexpected recovery watermarks: %v", spool.offsets)
	}
}

func TestQuotaRenewalExpiresWhileCompleteRecordIsPending(t *testing.T) {
	spool := admissionSpool{root: t.TempDir(), offsets: make(map[string]uint64)}
	collector := New(Config{MaxRecords: 1}, spool, &admissionReporter{})
	now := time.Now()
	collector.now = func() time.Time { return now }
	if err := collector.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spool.root, "a"), []byte("{}\n{}\npartial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := collector.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, allowed := collector.QuotaRenewal(); !allowed {
		t.Fatal("quota should renew before the backlog deadline")
	}
	now = now.Add(reportingGrace)
	if _, allowed := collector.QuotaRenewal(); allowed {
		t.Fatal("quota renewed with a complete record pending for five seconds")
	}
	if err := collector.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, allowed := collector.QuotaRenewal(); !allowed {
		t.Fatal("partial tail should not block recovery")
	}
}
