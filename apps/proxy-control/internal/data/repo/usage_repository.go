package repo

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
)

type UsageRepository struct{ db *gorm.DB }

func NewUsageRepository(db *gorm.DB) *UsageRepository {
	return &UsageRepository{db: db}
}

type usageTx struct{ db *gorm.DB }

func (r *UsageRepository) Transact(ctx context.Context, run func(usage.Transaction) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return run(usageTx{db: tx})
	})
}

func (t usageTx) Gateway(hash []byte) (string, string, bool, error) {
	var identity struct{ GatewayID, Status string }
	q := t.db.Raw(`SELECT t.gateway_id, g.status FROM gateway_tokens t JOIN gateways g ON g.id = t.gateway_id
		WHERE t.token_hash = ? AND t.scope = 'report' AND t.revoked_at IS NULL FOR SHARE OF t, g`, hash).Scan(&identity)
	return identity.GatewayID, identity.Status, q.RowsAffected != 0, q.Error
}
