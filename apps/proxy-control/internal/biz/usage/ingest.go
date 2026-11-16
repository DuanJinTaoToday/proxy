package usage

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

type sample struct {
	ClientID         string
	Network          string
	Src              string
	Dst              string
	Err              string
	Time             time.Time
	InputBytes       *int64
	OutputBytes      *int64
	InputBytesDelta  *int64
	OutputBytesDelta *int64
}

func parseSample(line string) (sample, string, bool) {
	var r sample
	if err := json.Unmarshal([]byte(line), &r); err != nil || r.Time.IsZero() || r.Time.Year() < 1 || r.Time.Year() > 9999 {
		return r, "invalid_record", false
	}
	if r.Network != "tcp" || r.ClientID == "" || r.Src == "" || r.Dst == "" || r.Err != "" {
		return r, "", false
	}
	if r.InputBytes == nil || r.OutputBytes == nil || r.InputBytesDelta == nil || r.OutputBytesDelta == nil ||
		*r.InputBytesDelta < 0 || *r.OutputBytesDelta < 0 || *r.InputBytesDelta > *r.InputBytes || *r.OutputBytesDelta > *r.OutputBytes {
		return r, "invalid_counters", false
	}
	return r, "", true
}

type ingestCursor struct {
	offset    int64
	updatedAt time.Time
}

type ingestPlan struct {
	cursors    map[string]*ingestCursor
	lastRecord time.Time
	tenants    map[string]int64
	clients    map[string]int64
	buckets    map[meteredKey]MeteredBucket
}

func validID(id string) bool {
	return id != "" && len(id) <= 128 && !strings.ContainsRune(id, 0)
}

func validOffsets(line RecorderLine) bool {
	start, end := line.FileOffset, line.EndOffset
	if !validID(line.FileID) || end <= start || end > math.MaxInt64 || strings.ContainsRune(line.Line, '\n') {
		return false
	}
	width := end - start
	n := uint64(len(line.Line))
	return width == n+1 || width == n+2
}

func (s *Service) SubmitBatch(ctx context.Context, token string, req Batch) (*BatchResult, error) {
	hash, err := reportToken(token)
	if err != nil {
		return nil, err
	}
	if !validID(req.GatewayID) || !validID(req.BootID) || len(req.Records) == 0 {
		return nil, invalidBatch()
	}
	if len(req.Records) > s.maxRecords {
		return nil, batchTooLarge()
	}
	reply := &BatchResult{}
	err = s.repo.Transact(ctx, func(tx Transaction) error {
		if err := authorizeGateway(tx, hash, req.GatewayID); err != nil {
			return err
		}
		beforeDate, err := tx.LockArchive()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.EnsureBoot(req.GatewayID, req.BootID, now); err != nil {
			return err
		}
		plan, err := planIngest(tx, req, reply, now)
		if err != nil {
			return err
		}
		if err := plan.meter(tx, req.GatewayID, beforeDate, now); err != nil {
			return err
		}
		return saveProgress(tx, req, plan)
	})
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func planIngest(tx Transaction, req Batch, reply *BatchResult, now time.Time) (ingestPlan, error) {
	p := ingestPlan{cursors: make(map[string]*ingestCursor), tenants: make(map[string]int64),
		clients: make(map[string]int64), buckets: make(map[meteredKey]MeteredBucket)}
	owners := make(map[string]string)
	ends := make(map[string]uint64)
	for _, line := range req.Records {
		fileID := line.FileID
		if !validOffsets(line) || line.FileOffset < ends[fileID] {
			return p, invalidBatch()
		}
		ends[fileID] = line.EndOffset
		cursor, err := p.cursor(tx, req, fileID)
		if err != nil {
			return p, err
		}
		if line.EndOffset <= uint64(cursor.offset) {
			reply.Duplicates++
			continue
		}
		if line.FileOffset != uint64(cursor.offset) {
			return p, invalidBatch()
		}
		cursor.offset = int64(line.EndOffset)
		cursor.updatedAt = now

		r, kind, qualified := parseSample(line.Line)
		if r.Time.After(p.lastRecord) && r.Time.Year() <= 9999 {
			p.lastRecord = r.Time
		}
		if qualified {
			if !validID(r.ClientID) {
				kind = "unknown_client"
			} else {
				tenant, known := owners[r.ClientID]
				if !known {
					tenant, err = tx.ClientTenant(r.ClientID)
					if err != nil {
						return p, err
					}
					owners[r.ClientID] = tenant
				}
				if tenant != "" {
					if err := p.record(tenant, r); err != nil {
						return p, err
					}
					reply.Billable++
					continue
				}
				kind = "unknown_client"
			}
		}
		if kind == "" {
			reply.NotBillable++
			continue
		}
		reply.Quarantined++
		if err := quarantine(tx, req, line, r, kind, now); err != nil {
			return p, err
		}
	}
	return p, nil
}

type meteredKey struct {
	client string
	minute time.Time
}

type MeteredBucket struct {
	TenantID, ClientID, UsageDate                       string
	MinuteStart                                         time.Time
	InputBytes, OutputBytes, BillableBytes, RecordCount int64
}

func (p ingestPlan) record(tenant string, r sample) error {
	input, output := *r.InputBytesDelta, *r.OutputBytesDelta
	total := input + output
	if p.tenants[tenant] > math.MaxInt64-total {
		return errors.BadRequest("BATCH_BYTE_OVERFLOW", "Recorder batch exceeds the signed byte range")
	}
	p.tenants[tenant] += total
	p.clients[r.ClientID] += total
	minute := r.Time.UTC().Truncate(time.Minute)
	key := meteredKey{r.ClientID, minute}
	bucket := p.buckets[key]
	bucket.TenantID, bucket.ClientID, bucket.UsageDate, bucket.MinuteStart = tenant, r.ClientID, r.Time.In(shanghai).Format(time.DateOnly), minute
	bucket.RecordCount++
	bucket.InputBytes += input
	bucket.OutputBytes += output
	bucket.BillableBytes += total
	p.buckets[key] = bucket
	return nil
}

func (p ingestPlan) meter(tx Transaction, gateway, beforeDate string, now time.Time) error {
	tenants := make([]string, 0, len(p.tenants))
	for tenant := range p.tenants {
		tenants = append(tenants, tenant)
	}
	slices.Sort(tenants)
	for _, tenant := range tenants {
		if err := tx.DebitTenant(tenant, p.tenants[tenant], now); err != nil {
			return err
		}
	}
	for client, total := range p.clients {
		if total > 0 {
			if err := tx.AddClientUsage(client, total); err != nil {
				return err
			}
		}
	}
	buckets := make([]MeteredBucket, 0, len(p.buckets))
	for _, bucket := range p.buckets {
		buckets = append(buckets, bucket)
	}
	return tx.WriteBuckets(gateway, buckets, beforeDate, now)
}

func quarantine(tx Transaction, req Batch, line RecorderLine, r sample, kind string, now time.Time) error {
	if !validID(r.ClientID) {
		r.ClientID = ""
	}
	detail, err := json.Marshal(struct {
		EndOffset   uint64 `json:"end_offset"`
		ClientID    string `json:"client_id,omitempty"`
		InputBytes  *int64 `json:"input_bytes,omitempty"`
		OutputBytes *int64 `json:"output_bytes,omitempty"`
		InputDelta  *int64 `json:"input_bytes_delta,omitempty"`
		OutputDelta *int64 `json:"output_bytes_delta,omitempty"`
	}{line.EndOffset, r.ClientID, r.InputBytes, r.OutputBytes, r.InputBytesDelta, r.OutputBytesDelta})
	if err != nil {
		return err
	}
	return tx.Quarantine(req.GatewayID, req.BootID, line.FileID, line.FileOffset, kind, string(detail), now)
}

func (p ingestPlan) cursor(tx Transaction, req Batch, fileID string) (*ingestCursor, error) {
	if cursor, ok := p.cursors[fileID]; ok {
		return cursor, nil
	}
	offset, err := tx.Cursor(req.GatewayID, req.BootID, fileID)
	cursor := &ingestCursor{offset: offset}
	p.cursors[fileID] = cursor
	return cursor, err
}

func saveProgress(tx Transaction, req Batch, p ingestPlan) error {
	for fileID, cursor := range p.cursors {
		if cursor.updatedAt.IsZero() {
			continue
		}
		if err := tx.SaveCursor(req.GatewayID, req.BootID, fileID, cursor.offset, cursor.updatedAt); err != nil {
			return err
		}
	}
	if !p.lastRecord.IsZero() {
		return tx.SaveLastRecord(req.GatewayID, req.BootID, p.lastRecord)
	}
	return nil
}
