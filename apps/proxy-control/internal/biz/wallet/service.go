package wallet

import (
	"context"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

const PageSize = 25

type Service struct {
	store  Repository
	access *access.Service
}

func NewService(store Repository, accessService *access.Service) *Service {
	return &Service{store: store, access: accessService}
}

func (s *Service) FindTenant(ctx context.Context, token, query string) (Tenant, error) {
	if _, err := s.access.AuthorizeOperator(ctx, token, access.ReadOperations); err != nil {
		return Tenant{}, err
	}
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 128 || strings.ContainsRune(query, 0) {
		return Tenant{}, ErrInvalid
	}
	return s.store.FindTenant(ctx, query, access.NormalizeEmail(query))
}

func (s *Service) Tenant(ctx context.Context, token, tenantID string, operations bool) (Tenant, error) {
	id, err := s.scope(ctx, token, tenantID, operations)
	if err != nil {
		return Tenant{}, err
	}
	return s.store.Tenant(ctx, id)
}

func (s *Service) CustomerWallet(ctx context.Context, token string) (Tenant, error) {
	id, err := s.scope(ctx, token, "", false)
	if err != nil {
		return Tenant{}, err
	}
	return s.store.CustomerWallet(ctx, id)
}

func (s *Service) QuoteRecharge(ctx context.Context, token, tenantID, key, reason string, amount int64) (Tenant, RechargeQuote, int64, error) {
	actor, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return Tenant{}, RechargeQuote{}, 0, err
	}
	reason = strings.TrimSpace(reason)
	before := int64(0)
	command := Command{TenantID: tenantID, Key: key, Action: Recharge, Amount: amount, Reason: reason, Actor: actor, ExpectedBefore: &before}
	if err := command.Validate(); err != nil {
		return Tenant{}, RechargeQuote{}, 0, err
	}
	tenant, err := s.store.Tenant(ctx, tenantID)
	if err != nil {
		return Tenant{}, RechargeQuote{}, 0, err
	}
	after, err := balanceAfter(tenant.AvailableMinor, amount)
	if err != nil {
		return Tenant{}, RechargeQuote{}, 0, err
	}
	quote := RechargeQuote{
		ID: uuid.NewString(), TenantID: tenant.ID, Actor: actor, RequestKey: key, Reason: reason,
		AmountMinor: amount, ExpectedBeforeMinor: tenant.AvailableMinor,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := s.store.CreateRechargeQuote(ctx, quote); err != nil {
		return Tenant{}, RechargeQuote{}, 0, err
	}
	return tenant, quote, after, nil
}

func (s *Service) Recharge(ctx context.Context, token, tenantID, key, quoteID string) (Entry, error) {
	actor, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return Entry{}, err
	}
	if _, err := uuid.Parse(quoteID); err != nil || len(quoteID) != 36 || tenantID == "" ||
		len(tenantID) > 64 || key == "" || len(key) > 128 || strings.ContainsRune(tenantID+key, 0) {
		return Entry{}, ErrInvalid
	}
	quote, err := s.store.RechargeQuote(ctx, tenantID, actor, key, quoteID)
	if err != nil {
		return Entry{}, err
	}
	command := Command{
		TenantID: tenantID, Key: key, Action: Recharge, Amount: quote.AmountMinor, Actor: actor, Reason: quote.Reason,
		ExpectedBefore: &quote.ExpectedBeforeMinor, QuoteExpiresAt: quote.ExpiresAt,
	}
	if err := command.Validate(); err != nil {
		return Entry{}, err
	}
	return s.store.ApplyRecharge(ctx, command)
}

func (s *Service) Entries(ctx context.Context, token, tenantID string, operations bool, filter Filter) ([]Entry, error) {
	id, err := s.scope(ctx, token, tenantID, operations)
	if err != nil {
		return nil, err
	}
	switch Action(filter.Kind) {
	case "", Recharge, Debit, Hold, Release, Refund:
	default:
		return nil, ErrInvalid
	}
	if filter.BeforeID < 0 || !filter.validDates() {
		return nil, ErrInvalid
	}
	filter.Limit = PageSize + 1
	return s.store.Entries(ctx, id, filter)
}

func (s *Service) PurchasePackage(ctx context.Context, token string, purchase PackagePurchase) (Order, error) {
	id, err := s.scope(ctx, token, "", false)
	if err != nil {
		return Order{}, err
	}
	if purchase.PackageID == "" || len(purchase.PackageID) > 128 || purchase.Version <= 0 ||
		purchase.Key == "" || len(purchase.Key) > 128 || strings.ContainsRune(purchase.PackageID+purchase.Key, 0) {
		return Order{}, ErrInvalid
	}
	return s.store.PurchasePackage(ctx, id, purchase)
}

func (s *Service) Orders(ctx context.Context, token, tenantID string, operations bool, filter Filter) (OrderPage, error) {
	id, err := s.scope(ctx, token, tenantID, operations)
	if err != nil {
		return OrderPage{}, err
	}
	if filter.BeforeID < 0 || !filter.validOrderQuery() || !filter.validDates() ||
		(filter.Kind != "" && !IsKind(filter.Kind)) ||
		(filter.Status != "" && !IsStatus(filter.Status)) ||
		(filter.Product != "" && filter.Product != "dynamic" && filter.Product != "static") {
		return OrderPage{}, ErrInvalid
	}
	filter.Limit = PageSize + 1
	return s.store.Orders(ctx, id, filter)
}

func (s *Service) Order(ctx context.Context, token, tenantID string, operations bool, orderID int64) (Order, error) {
	id, err := s.scope(ctx, token, tenantID, operations)
	if err != nil {
		return Order{}, err
	}
	if orderID <= 0 {
		return Order{}, ErrInvalid
	}
	return s.store.OrderDetail(ctx, id, orderID)
}

func (s *Service) scope(ctx context.Context, token, tenantID string, operations bool) (string, error) {
	if !operations {
		tenant, err := s.access.Account(ctx, token)
		return tenant.ID, err
	}
	if _, err := s.access.AuthorizeOperator(ctx, token, access.ReadOperations); err != nil {
		return "", err
	}
	if tenantID == "" || len(tenantID) > 64 || strings.ContainsRune(tenantID, 0) {
		return "", ErrInvalid
	}
	return tenantID, nil
}

func PublicError(err error) error {
	if err == nil {
		return nil
	}
	var public *errors.Error
	if errors.As(err, &public) {
		return public
	}
	return errors.ServiceUnavailable("WALLET_UNAVAILABLE", "wallet service unavailable")
}
