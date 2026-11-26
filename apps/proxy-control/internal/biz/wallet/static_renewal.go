package wallet

import (
	"context"
	"slices"

	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

const MaxStaticRenewalItems = 25

var (
	ErrStaticRenewalPending = errors.Conflict("STATIC_RENEWAL_PENDING", "an allocation is already pending renewal")
	ErrStaticRenewalExpired = errors.Conflict("STATIC_RENEWAL_EXPIRED", "an allocation has expired")
	ErrStaticRenewalPrice   = errors.Conflict("STATIC_RENEWAL_PRICE_UNAVAILABLE", "no confirmed renewal price for this allocation and term")
)

type SupplierRenewal struct {
	Expiries map[string]time.Time
	Rejected bool
}

type StaticRenewalRequest struct {
	IDs               []string
	Term              string
	Key               string
	ConfirmationToken string
}

type StaticRenewalLine struct {
	ID             string
	IP             string
	UnitMinor      int64
	TermDays       int32
	OriginalExpiry time.Time
}

type StaticRenewalTerm struct {
	Term     string
	TermDays int32
}

type StaticRenewalQuote struct {
	Lines             []StaticRenewalLine
	Term              string
	ConfirmationToken string
	ExpiresAt         time.Time
}

type StaticRenewalResolution struct {
	TenantID         string
	OrderID          int64
	ItemID           string
	Evidence         string
	CloseUnknown     bool
	ConfirmedFailure bool
}

type StaticRenewalRepository interface {
	ListStaticRenewalTerms(context.Context, string, []string) ([]StaticRenewalTerm, error)
	QuoteStaticRenewal(context.Context, string, StaticRenewalRequest) (StaticRenewalQuote, error)
	RenewStatic(context.Context, string, StaticRenewalRequest) (Order, error)
	RecheckStaticRenewal(context.Context, string, int64) (Order, error)
	ResolveStaticRenewal(context.Context, string, StaticRenewalResolution) (Order, error)
}

func (s *Service) ListStaticRenewalTerms(ctx context.Context, token string, ids []string) ([]StaticRenewalTerm, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return nil, err
	}
	ids, err = normalizeRenewalIDs(ids)
	if err != nil {
		return nil, err
	}
	return s.store.ListStaticRenewalTerms(ctx, tenant, ids)
}

func (s *Service) QuoteStaticRenewal(ctx context.Context, token string, p StaticRenewalRequest) (StaticRenewalQuote, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return StaticRenewalQuote{}, err
	}
	if err := p.normalize(); err != nil {
		return StaticRenewalQuote{}, err
	}
	return s.store.QuoteStaticRenewal(ctx, tenant, p)
}

func (s *Service) RenewStatic(ctx context.Context, token string, p StaticRenewalRequest) (Order, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return Order{}, err
	}
	if err := p.normalize(); err != nil || p.Key == "" || len(p.Key) > 128 || strings.ContainsRune(p.Key, 0) ||
		len(p.ConfirmationToken) == 0 || len(p.ConfirmationToken) > 16384 {
		return Order{}, ErrInvalid
	}
	return s.store.RenewStatic(ctx, tenant, p)
}

func (s *Service) RecheckStaticRenewal(ctx context.Context, token string, orderID int64) (Order, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return Order{}, err
	}
	if orderID <= 0 {
		return Order{}, ErrInvalid
	}
	return s.store.RecheckStaticRenewal(ctx, tenant, orderID)
}

func (s *Service) ResolveStaticRenewal(ctx context.Context, token string, p StaticRenewalResolution) (Order, error) {
	actor, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return Order{}, err
	}
	if p.TenantID == "" || len(p.TenantID) > 64 || p.OrderID <= 0 || p.ItemID == "" || len(p.ItemID) > 128 ||
		strings.TrimSpace(p.Evidence) == "" || len(p.Evidence) > 500 || strings.ContainsRune(p.TenantID+p.ItemID+p.Evidence, 0) ||
		(p.CloseUnknown && p.ConfirmedFailure) {
		return Order{}, ErrInvalid
	}
	return s.store.ResolveStaticRenewal(ctx, actor, p)
}

func (p *StaticRenewalRequest) normalize() error {
	if p.Term == "" || len(p.Term) > 16 || strings.ContainsRune(p.Term, 0) {
		return ErrInvalid
	}
	var err error
	p.IDs, err = normalizeRenewalIDs(p.IDs)
	return err
}

func normalizeRenewalIDs(ids []string) ([]string, error) {
	if len(ids) == 0 || len(ids) > MaxStaticRenewalItems {
		return nil, ErrInvalid
	}
	ids = slices.Clone(ids)
	slices.Sort(ids)
	for i, id := range ids {
		if id == "" || len(id) > 128 || strings.ContainsRune(id, 0) || (i > 0 && ids[i-1] == id) {
			return nil, ErrInvalid
		}
	}
	return ids, nil
}

func (o Order) SettleRenewal(before int64, outcomes []string) (Settlement, error) {
	if o.Kind != StaticRenewal || o.Status != Pending || len(outcomes) != len(o.Items) {
		return Settlement{}, ErrState
	}
	previouslyReleased := o.ReleasedMinor
	o.ReleasedMinor = 0
	o.DeliveredCount = 0
	pending := false
	for index, outcome := range outcomes {
		item := &o.Items[index]
		switch outcome {
		case Succeeded:
			item.DeliveredCount = 1
			o.DeliveredCount++
		case Failed:
			o.ReleasedMinor += item.UnitMinor
		case Pending:
			pending = true
		default:
			return Settlement{}, ErrState
		}
	}
	if !pending {
		o.Status = Partial
		switch o.DeliveredCount {
		case 0:
			o.Status = Failed
		case int64(len(o.Items)):
			o.Status = Succeeded
		}
	}
	after, err := balanceAfter(before, o.ReleasedMinor-previouslyReleased)
	return Settlement{AfterMinor: after, Order: o}, err
}
