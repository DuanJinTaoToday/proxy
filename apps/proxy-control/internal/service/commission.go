package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Wallet) GetCommissionRates(ctx context.Context, _ *v1.BaseRequest) (*v1.CommissionRates, error) {
	noStore(ctx)
	result, err := s.biz.CommissionRates(ctx, bearerToken(ctx))
	return commissionRatesMessage(result, err)
}

func (s *Wallet) SetCommissionRates(ctx context.Context, req *v1.SetCommissionRatesRequest) (*v1.CommissionRates, error) {
	noStore(ctx)
	result, err := s.biz.SetCommissionRates(ctx, bearerToken(ctx), req.FirstPurchaseBps, req.RepeatPurchaseBps)
	return commissionRatesMessage(result, err)
}

func commissionRatesMessage(rates wallet.CommissionRates, err error) (*v1.CommissionRates, error) {
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	return &v1.CommissionRates{
		Version: rates.Version, FirstPurchaseBps: rates.FirstPurchaseBPS, RepeatPurchaseBps: rates.RepeatPurchaseBPS,
		OperatorId: rates.OperatorID, UpdatedAt: timestamppb.New(rates.UpdatedAt),
	}, nil
}

func (s *Wallet) ListTenantCommissions(ctx context.Context, req *v1.TenantCommissionsRequest) (*v1.CommissionPage, error) {
	return s.listCommissions(ctx, "", false, wallet.CommissionFilter{BeforeID: req.GetBeforeId(), CustomerKey: req.GetCustomerKey()})
}

func (s *Wallet) ListOperationsCommissions(ctx context.Context, req *v1.OperationsCommissionsRequest) (*v1.CommissionPage, error) {
	return s.listCommissions(ctx, req.GetTenantId(), true, wallet.CommissionFilter{BeforeID: req.GetBeforeId(), CustomerKey: req.GetCustomerKey()})
}

func (s *Wallet) listCommissions(ctx context.Context, tenant string, operations bool, filter wallet.CommissionFilter) (*v1.CommissionPage, error) {
	noStore(ctx)
	page, err := s.biz.Commissions(ctx, bearerToken(ctx), tenant, operations, filter)
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	response := &v1.CommissionPage{AvailableMinor: page.AvailableMinor}
	if len(page.Records) > wallet.PageSize {
		page.Records = page.Records[:wallet.PageSize]
		response.NextBeforeId = page.Records[len(page.Records)-1].ID
	}
	for _, r := range page.Records {
		response.Records = append(response.Records, &v1.CommissionRecord{
			Id: r.ID, CustomerKey: r.CustomerKey, ReferralSource: r.ReferralSource, ProductKind: r.ProductKind,
			SettlementSequence: r.SettlementSequence, Classification: r.Classification(), SettledAt: timestamppb.New(r.SettledAt),
			NetDebitMinor: r.NetDebitMinor, CommissionMinor: r.CommissionMinor, RateBps: r.RateBPS, RateVersion: r.RateVersion,
			RuleVersion: r.RuleVersion, Status: r.Status(),
		})
	}
	return response, nil
}
