package usage

import (
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/go-kratos/kratos/v3/errors"
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
	Require(string, string, access.Permission) error
	RequireActor(string, string, access.Permission) (string, error)
}

type Service struct {
	repo       Repository
	access     TenantAccess
	maxRecords int
	cacheMu    sync.Mutex
	cache      map[string]cachedBalance
}

func New(repo Repository, access TenantAccess) *Service {
	return &Service{repo: repo, access: access, maxRecords: DefaultBatchRecords, cache: make(map[string]cachedBalance)}
}

func NewConfigured(settings *conf.Biz, repo Repository, access TenantAccess) *Service {
	service := New(repo, access)
	if limit := int(settings.GetUsage().GetMaxBatchRecords()); limit > 0 {
		service.maxRecords = limit
	}
	return service
}

func reportToken(value string) ([]byte, error) {
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil && len(decoded) >= 32 {
		hash := sha256.Sum256([]byte(value))
		return hash[:], nil
	}
	return nil, errors.Unauthorized("GATEWAY_UNAUTHENTICATED", "valid report token required")
}

func authorizeGateway(tx Transaction, hash []byte, gatewayID string) error {
	id, status, found, err := tx.Gateway(hash)
	if err != nil {
		return err
	}
	if !found {
		return errors.Unauthorized("GATEWAY_UNAUTHENTICATED", "valid report token required")
	}
	if id != gatewayID || status != "active" {
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
