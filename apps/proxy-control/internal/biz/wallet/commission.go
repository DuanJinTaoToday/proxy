package wallet

import (
	"context"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

const MaxCommissionRateBPS = 10000

var ErrCommissionRates = errors.BadRequest("INVALID_COMMISSION_RATES", "both commission rates must be whole basis points from 0 to 10000")
var ErrCommissionBalance = errors.Conflict("COMMISSION_BALANCE_INSUFFICIENT", "available commission is insufficient")

type CommissionRates struct {
	Version                             int64
	FirstPurchaseBPS, RepeatPurchaseBPS int32
	OperatorID                          string
	UpdatedAt                           time.Time
}

func (r CommissionRates) ForSettlement(sequence int64) int32 {
	if sequence == 1 {
		return r.FirstPurchaseBPS
	}
	return r.RepeatPurchaseBPS
}

func CommissionMinor(netMinor int64, basisPoints int32) int64 {
	// Split before multiplying so the full signed monetary range remains safe.
	return netMinor/MaxCommissionRateBPS*int64(basisPoints) +
		(netMinor%MaxCommissionRateBPS*int64(basisPoints)+MaxCommissionRateBPS/2)/MaxCommissionRateBPS
}

type CommissionRecord struct {
	ID, SettlementSequence, NetDebitMinor, CommissionMinor, RateVersion int64
	RateBPS, RuleVersion                                                int32
	CustomerKey, ReferralSource, ProductKind                            string
	PromotionPaused                                                     bool
	SettledAt                                                           time.Time
}

func (r CommissionRecord) Classification() string {
	if r.SettlementSequence == 1 {
		return "first_purchase"
	}
	return "repeat_purchase"
}

func (r CommissionRecord) Status() string {
	switch {
	case r.PromotionPaused:
		return "promotion_paused"
	case r.CommissionMinor == 0:
		return "zero_commission"
	default:
		return "credited"
	}
}

type CommissionFilter struct {
	BeforeID    int64
	CustomerKey string
	Limit       int
}

type CommissionPage struct {
	AvailableMinor int64
	Records        []CommissionRecord
}

type CommissionRepository interface {
	CommissionRates(context.Context) (CommissionRates, error)
	SetCommissionRates(context.Context, string, CommissionRates) (CommissionRates, error)
	Commissions(context.Context, string, CommissionFilter) (CommissionPage, error)
	TransferCommission(context.Context, Command) (Entry, error)
}

func (s *Service) CommissionRates(ctx context.Context, token string) (CommissionRates, error) {
	if _, err := s.access.AuthorizeOperator(ctx, token, access.ReadOperations); err != nil {
		return CommissionRates{}, err
	}
	return s.store.CommissionRates(ctx)
}

func (s *Service) SetCommissionRates(ctx context.Context, token string, first, repeat *int32) (CommissionRates, error) {
	actor, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return CommissionRates{}, err
	}
	if first == nil || repeat == nil || *first < 0 || *first > MaxCommissionRateBPS ||
		*repeat < 0 || *repeat > MaxCommissionRateBPS {
		return CommissionRates{}, ErrCommissionRates
	}
	return s.store.SetCommissionRates(ctx, actor, CommissionRates{FirstPurchaseBPS: *first, RepeatPurchaseBPS: *repeat})
}

func (s *Service) Commissions(ctx context.Context, token, tenantID string, operations bool, filter CommissionFilter) (CommissionPage, error) {
	id, err := s.scope(ctx, token, tenantID, operations)
	if err != nil {
		return CommissionPage{}, err
	}
	if filter.BeforeID < 0 {
		return CommissionPage{}, ErrInvalid
	}
	if filter.CustomerKey != "" {
		key, err := uuid.Parse(filter.CustomerKey)
		if err != nil || len(filter.CustomerKey) != 36 {
			return CommissionPage{}, ErrInvalid
		}
		filter.CustomerKey = key.String()
	}
	filter.Limit = PageSize + 1
	return s.store.Commissions(ctx, id, filter)
}

func (s *Service) TransferCommission(ctx context.Context, token, key string, amount int64) (Entry, error) {
	id, err := s.scope(ctx, token, "", false)
	if err != nil {
		return Entry{}, err
	}
	command := Command{TenantID: id, Key: key, Action: CommissionTransfer, Amount: amount}
	if err := command.Validate(); err != nil {
		return Entry{}, err
	}
	return s.store.TransferCommission(ctx, command)
}
