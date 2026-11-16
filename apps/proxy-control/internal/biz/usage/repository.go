package usage

import (
	"context"
	"time"
)

type Repository interface {
	Transact(ctx context.Context, run func(Transaction) error) error
	QueryUsageBuckets(ctx context.Context, query UsageQuery, after *UsageKey, limit int) ([]UsageBucket, error)
	ListRecordAnomalies(ctx context.Context, filter FindingFilter, after *FindingKey, limit int) ([]RecordAnomaly, error)
	ListUsageGaps(ctx context.Context, filter FindingFilter, after *FindingKey, limit int) ([]UsageGap, error)
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
	Quarantine(gatewayID, bootID, fileID string, offset uint64, kind, detail string, now time.Time) error
	ClientTenant(clientID string) (string, error)
	DebitTenant(tenantID string, bytes int64, now time.Time) error
	AddClientUsage(clientID string, bytes int64) error
	WriteBuckets(gatewayID string, buckets []MeteredBucket, beforeDate string, now time.Time) error
	DatesBefore(cutoff string) ([]string, error)
	ArchiveDay(date string) error
	SetArchiveCutoff(cutoff string) error
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
