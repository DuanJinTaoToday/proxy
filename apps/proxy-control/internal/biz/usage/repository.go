package usage

import (
	"context"
	"time"
)

type Repository interface {
	Transact(ctx context.Context, run func(Transaction) error) error
	DailyUsage(ctx context.Context, tenantID string, start, end time.Time) ([]DailyTotals, *time.Time, error)
	QueryUsageBuckets(ctx context.Context, query UsageQuery, after *UsageKey, limit int) ([]UsageBucket, error)
	ListRecordAnomalies(ctx context.Context, filter FindingFilter, after *FindingKey, limit int) ([]RecordAnomaly, error)
	ListUsageGaps(ctx context.Context, filter FindingFilter, after *FindingKey, limit int) ([]UsageGap, error)
	AuditGrants(ctx context.Context, tenant string, after int64, limit int) ([]AuditedGrant, int64, error)
	AuditLedger(ctx context.Context, tenant string, after int64, limit int) ([]LedgerEntry, error)
	AuditRates(ctx context.Context, scopeType, scopeID string, after int64, limit int) ([]RateVersion, error)
	DueTenants(ctx context.Context, now time.Time) ([]string, error)
}

// Transaction keeps the locks, usage writes and cursor updates in the same commit.
type Transaction interface {
	Gateway(hash []byte) (id, status string, found bool, err error)
	LockArchive() (string, error)
	EnsureBoot(gatewayID, bootID string, now time.Time) error
	ReportLifecycle(gatewayID, bootID string, now time.Time, heartbeat, exit *time.Time) error
	LockReplay(gatewayID, bootID string) (exited, completed bool, err error)
	SaveSessionGap(gatewayID, bootID string, session UnfinishedSession, now time.Time) error
	CountSessionGaps(gatewayID, bootID string) (int64, error)
	FinishReplay(gatewayID, bootID string, hasSessionRecords *bool, now time.Time) error
	Cursor(gatewayID, bootID, fileID string) (int64, error)
	SaveCursor(gatewayID, bootID, fileID string, offset int64, now time.Time) error
	SaveLastRecord(gatewayID, bootID string, recordedAt time.Time) error
	TenantForClient(clientID string) (string, error)
	Quarantine(gatewayID, bootID, fileID string, offset uint64, kind, detail string, now time.Time) error
	LoadAccount(id string, now time.Time) (Account, error)
	Thresholds(id string, now time.Time) ([]Threshold, error)
	SaveGrant(Grant) error
	SaveAccount(Account, time.Time) error
	Rates([]RateSample) ([]Rate, error)
	WriteBuckets([]Bucket, []Bucket) error
	WriteLedger(tenant string, grantID int64, kind string, delta, before, after int64, correction *int64, now time.Time) error
	ExpiryEntry(grantID int64) (int64, bool, error)
	WriteExpiry(tenant string, grantID, delta, available int64, now time.Time) error
	DatesBefore(cutoff string) ([]string, error)
	ArchiveDay(date string) error
	SetArchiveCutoff(cutoff string) error
}

type Threshold struct{ AdmissionThresholdBytes, WarningThresholdBytes *int64 }

type RateSample struct {
	TenantID   string
	RecordedAt time.Time
}

type Rate struct {
	ID, UploadRatioMilli, DownloadRatioMilli int64
}

type RecorderLine struct {
	FileID                string
	FileOffset, EndOffset uint64
	Line                  string
}

type Batch struct {
	GatewayID, BootID string
	Records           []RecorderLine
}

type BatchResult struct {
	Billable, NotBillable, Quarantined, Duplicates uint32
}

type DailyTotals struct {
	Date                                   string
	InputBytes, OutputBytes, BillableBytes uint64
}

type DailyUsage struct {
	Days           []Day
	LastReceivedAt *time.Time
}

type Day struct {
	DailyTotals
	TotalBytes uint64
}

type Balance struct {
	TenantID                                                       string
	AvailableBytes                                                 int64
	OverdraftBytes, AdmissionThresholdBytes, WarningThresholdBytes uint64
	WarningSince                                                   *time.Time
	Grants                                                         []Grant
}
