package usage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"gorm.io/gorm"
)

const DefaultBatchRecords = 1000

var shanghai = time.UTC

func init() {
	var err error
	shanghai, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
}

type TenantAccess interface {
	Require(context.Context, string, v1.Permission) error
}

type Service struct {
	db         *gorm.DB
	access     TenantAccess
	maxRecords int
	cacheMu    sync.Mutex
	cache      map[string]cachedBalance
}

func New(db *gorm.DB, access TenantAccess) *Service {
	return &Service{db: db, access: access, maxRecords: DefaultBatchRecords, cache: make(map[string]cachedBalance)}
}

func (s *Service) SetMaxRecords(limit int) {
	if limit > 0 {
		s.maxRecords = limit
	}
}

func reportToken(ctx context.Context) ([]byte, error) {
	tr, ok := transport.FromServerContext(ctx)
	if ok {
		scheme, value, _ := strings.Cut(tr.RequestHeader().Get("Authorization"), " ")
		if strings.EqualFold(scheme, "Bearer") && len(value) <= 256 {
			if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil && len(decoded) >= 32 {
				hash := sha256.Sum256([]byte(value))
				return hash[:], nil
			}
		}
	}
	return nil, errors.Unauthorized("GATEWAY_UNAUTHENTICATED", "valid report token required")
}

func authorizeGateway(tx *gorm.DB, hash []byte, gatewayID string) error {
	var identity struct{ GatewayID, Status string }
	q := tx.Raw(`SELECT t.gateway_id, g.status FROM gateway_tokens t JOIN gateways g ON g.id = t.gateway_id
		WHERE t.token_hash = ? AND t.scope = 'report' AND t.revoked_at IS NULL FOR SHARE OF t, g`, hash).Scan(&identity)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return errors.Unauthorized("GATEWAY_UNAUTHENTICATED", "valid report token required")
	}
	if identity.GatewayID != gatewayID || identity.Status != "active" {
		return errors.Forbidden("GATEWAY_FORBIDDEN", "gateway is disabled or does not match the token")
	}
	return nil
}

func invalidBatch() error {
	return errors.BadRequest("INVALID_BATCH", "invalid Recorder batch or non-contiguous file offsets")
}

func batchTooLarge() error {
	return errors.New(413, "BATCH_TOO_LARGE", "Recorder batch exceeds the record limit")
}
