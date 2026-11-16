package overview

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/subaccount"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
)

type Service struct {
	wallet      *wallet.Service
	usage       *usage.Service
	product     *product.Service
	subaccounts *subaccount.Service
}

func New(w *wallet.Service, u *usage.Service, p *product.Service, s *subaccount.Service) *Service {
	return &Service{wallet: w, usage: u, product: p, subaccounts: s}
}

type Summary struct {
	WalletMinor, TrafficBytes int64
	Static                    wallet.StaticSummary
	LowTraffic                bool
	Products                  []product.Product
	CanCreateSubaccount       bool
}

func (s *Service) Get(ctx context.Context, token string) (Summary, error) {
	account, err := s.wallet.Tenant(ctx, token, "", false)
	if err != nil {
		return Summary{}, wallet.PublicError(err)
	}
	traffic, err := s.usage.TrafficAlert(ctx, token)
	if err != nil {
		return Summary{}, err
	}
	static, err := s.wallet.StaticSummary(ctx, token)
	if err != nil {
		return Summary{}, wallet.PublicError(err)
	}
	accounts, err := s.subaccounts.List(ctx, token)
	if err != nil {
		return Summary{}, err
	}
	products, err := s.product.Products(ctx)
	if err != nil {
		return Summary{}, product.PublicError(err)
	}
	return Summary{
		WalletMinor: account.AvailableMinor, TrafficBytes: traffic.AvailableBytes,
		Static: static, LowTraffic: traffic.TriggeredAt != nil, Products: products,
		CanCreateSubaccount: len(accounts) < subaccount.MaxAccounts,
	}, nil
}
