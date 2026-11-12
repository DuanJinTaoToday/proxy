package usage

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"slices"
	"strings"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/model"
	"github.com/go-kratos/kratos/v3/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

type ingestPlan struct {
	records    []record
	cursors    map[string]*model.IngestCursor
	lastRecord time.Time
}

func validID(id string) bool {
	return id != "" && len(id) <= 128 && !strings.ContainsRune(id, 0)
}

func validOffsets(line *v1.RecorderLine) bool {
	start, end := line.GetFileOffset(), line.GetEndOffset()
	if !validID(line.GetFileId()) || end <= start || end > math.MaxInt64 || strings.ContainsRune(line.GetLine(), '\n') {
		return false
	}
	width := end - start
	n := uint64(len(line.GetLine()))
	return width == n+1 || width == n+2
}

func (s *Service) SubmitBatch(ctx context.Context, req *v1.SubmitBatchRequest) (*v1.SubmitBatchResponse, error) {
	hash, err := reportToken(ctx)
	if err != nil {
		return nil, err
	}
	if !validID(req.GetGatewayId()) || !validID(req.GetBootId()) || len(req.GetRecords()) == 0 {
		return nil, invalidBatch()
	}
	if len(req.GetRecords()) > s.maxRecords {
		return nil, batchTooLarge()
	}
	reply := &v1.SubmitBatchResponse{}
	var changed []string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizeGateway(tx, hash, req.GetGatewayId()); err != nil {
			return err
		}
		before, err := lockArchiveState(tx)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Exec(`INSERT INTO gateway_boots (gateway_id, boot_id, first_seen_at)
			VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, req.GetGatewayId(), req.GetBootId(), now).Error; err != nil {
			return err
		}
		plan, err := planIngest(tx, req, reply, now)
		if err != nil {
			return err
		}
		changed, err = applyRecords(tx, req.GetGatewayId(), plan.records, before, now)
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

func planIngest(tx *gorm.DB, req *v1.SubmitBatchRequest, reply *v1.SubmitBatchResponse, now time.Time) (ingestPlan, error) {
	p := ingestPlan{cursors: make(map[string]*model.IngestCursor)}
	mappings := make(map[string]string)
	ends := make(map[string]uint64)
	for _, line := range req.GetRecords() {
		fileID := line.GetFileId()
		if !validOffsets(line) || line.GetFileOffset() < ends[fileID] {
			return p, invalidBatch()
		}
		ends[fileID] = line.GetEndOffset()
		cursor, err := p.cursor(tx, req, fileID)
		if err != nil {
			return p, err
		}
		if line.GetEndOffset() <= uint64(cursor.ConfirmedOffset) {
			reply.Duplicates++
			continue
		}
		if line.GetFileOffset() != uint64(cursor.ConfirmedOffset) {
			return p, invalidBatch()
		}
		cursor.ConfirmedOffset = int64(line.GetEndOffset())
		cursor.UpdatedAt = now

		r, kind, qualified := parseSample(line.GetLine())
		if r.Time.After(p.lastRecord) && r.Time.Year() <= 9999 {
			p.lastRecord = r.Time
		}
		if qualified {
			tenantID, ok := mappings[r.ClientID]
			if !ok {
				if validID(r.ClientID) {
					err = tx.Raw("SELECT tenant_id FROM proxy_credentials WHERE client_id = ?", r.ClientID).Scan(&tenantID).Error
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

func quarantine(tx *gorm.DB, req *v1.SubmitBatchRequest, line *v1.RecorderLine, r sample, kind string, now time.Time) error {
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
	}{line.GetEndOffset(), r.ClientID, r.InputBytes, r.OutputBytes, r.InputBytesDelta, r.OutputBytesDelta})
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO record_anomalies (gateway_id, boot_id, file_id, file_offset, kind, detail, detected_at)
		VALUES (?, ?, ?, ?, ?, ?::jsonb, ?)`, req.GetGatewayId(), req.GetBootId(), line.GetFileId(), line.GetFileOffset(), kind, string(detail), now).Error
}

func (p ingestPlan) cursor(tx *gorm.DB, req *v1.SubmitBatchRequest, fileID string) (*model.IngestCursor, error) {
	if cursor, ok := p.cursors[fileID]; ok {
		return cursor, nil
	}
	cursor := &model.IngestCursor{GatewayID: req.GetGatewayId(), BootID: req.GetBootId(), FileID: fileID}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("gateway_id = ? AND boot_id = ? AND file_id = ?", cursor.GatewayID, cursor.BootID, fileID).Find(cursor).Error
	p.cursors[fileID] = cursor
	return cursor, err
}

func saveProgress(tx *gorm.DB, req *v1.SubmitBatchRequest, p ingestPlan) error {
	for _, cursor := range p.cursors {
		if cursor.UpdatedAt.IsZero() {
			continue
		}
		if err := tx.Exec(`INSERT INTO ingest_cursors (gateway_id, boot_id, file_id, confirmed_offset, updated_at)
			VALUES (?, ?, ?, ?, ?) ON CONFLICT (gateway_id, boot_id, file_id) DO UPDATE
			SET confirmed_offset = EXCLUDED.confirmed_offset, updated_at = EXCLUDED.updated_at`,
			cursor.GatewayID, cursor.BootID, cursor.FileID, cursor.ConfirmedOffset, cursor.UpdatedAt).Error; err != nil {
			return err
		}
	}
	if !p.lastRecord.IsZero() {
		return tx.Exec(`UPDATE gateway_boots SET last_record_at = GREATEST(last_record_at, ?)
			WHERE gateway_id = ? AND boot_id = ?`, p.lastRecord, req.GetGatewayId(), req.GetBootId()).Error
	}
	return nil
}

func applyRecords(tx *gorm.DB, gatewayID string, records []record, before string, now time.Time) ([]string, error) {
	tenants := make(map[string]*account)
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
	buckets := make(map[bucketKey]bucket)
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
		if a.lastReceivedAt == nil || a.lastReceivedAt.Before(now) {
			a.lastReceivedAt = &now
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
