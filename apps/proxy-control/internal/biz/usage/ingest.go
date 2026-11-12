package usage

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
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

type record struct {
	sample
	tenantID string
}

type ingestCursor struct {
	offset    int64
	updatedAt time.Time
}

type ingestPlan struct {
	records    []record
	cursors    map[string]*ingestCursor
	lastRecord time.Time
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
	var changed []string
	err = s.repo.Transact(ctx, func(tx Transaction) error {
		if err := authorizeGateway(tx, hash, req.GatewayID); err != nil {
			return err
		}
		before, err := tx.LockArchive()
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
		changed, err = applyRecords(tx, req.GatewayID, plan.records, before, now)
		if err != nil {
			return err
		}
		return saveProgress(tx, req, plan)
	})
	if err != nil {
		return nil, err
	}
	s.refreshBalances(ctx, changed)
	return reply, nil
}

func planIngest(tx Transaction, req Batch, reply *BatchResult, now time.Time) (ingestPlan, error) {
	p := ingestPlan{cursors: make(map[string]*ingestCursor)}
	mappings := make(map[string]string)
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
			tenantID, ok := mappings[r.ClientID]
			if !ok {
				if validID(r.ClientID) {
					tenantID, err = tx.TenantForClient(r.ClientID)
					if err != nil {
						return p, err
					}
				}
				mappings[r.ClientID] = tenantID
			}
			if tenantID != "" {
				p.records = append(p.records, record{sample: r, tenantID: tenantID})
				reply.Billable++
				continue
			}
			kind = "unknown_client_id"
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

func applyRecords(tx Transaction, gatewayID string, records []record, before string, now time.Time) ([]string, error) {
	tenants := make(map[string]*Account)
	var ids []string
	for _, r := range records {
		if _, ok := tenants[r.tenantID]; !ok {
			tenants[r.tenantID] = nil
			ids = append(ids, r.tenantID)
		}
	}
	slices.Sort(ids)
	for _, id := range ids {
		a, err := loadAccount(tx, id, now)
		if err != nil {
			return nil, err
		}
		tenants[id] = a
	}

	rates, err := resolveRates(tx, records)
	if err != nil {
		return nil, err
	}
	buckets := make(map[bucketKey]Bucket)
	for i, r := range records {
		if err := allocateRecord(tx, tenants[r.tenantID], r, rates[i], before, now, buckets); err != nil {
			return nil, err
		}
	}
	if err := writeBuckets(tx, gatewayID, buckets, now); err != nil {
		return nil, err
	}
	for _, id := range ids {
		a := tenants[id]
		if a.LastReceivedAt == nil || a.LastReceivedAt.Before(now) {
			a.LastReceivedAt = &now
		}
		if err := a.save(tx, now); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func billableBytes(input, output, upload, download int64) (int64, error) {
	var left, right big.Int
	left.Mul(big.NewInt(input), big.NewInt(upload))
	right.Mul(big.NewInt(output), big.NewInt(download))
	left.Add(&left, &right)
	left.Add(&left, big.NewInt(999))
	left.Quo(&left, big.NewInt(1000))
	if !left.IsInt64() {
		return 0, errors.BadRequest("INVALID_BATCH", "billable byte count exceeds storage range")
	}
	return left.Int64(), nil
}
