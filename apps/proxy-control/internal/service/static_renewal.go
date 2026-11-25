package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Wallet) ListStaticRenewalTerms(ctx context.Context, req *v1.StaticRenewalTermsRequest) (*v1.StaticRenewalTerms, error) {
	noStore(ctx)
	terms, err := s.biz.ListStaticRenewalTerms(ctx, bearerToken(ctx), req.GetAllocationIds())
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	result := &v1.StaticRenewalTerms{}
	for _, term := range terms {
		result.Terms = append(result.Terms, &v1.StaticRenewalTerm{Term: term.Term, TermDays: term.TermDays})
	}
	return result, nil
}

func (s *Wallet) QuoteStaticRenewal(ctx context.Context, req *v1.StaticRenewalQuoteRequest) (*v1.StaticRenewalQuote, error) {
	noStore(ctx)
	quote, err := s.biz.QuoteStaticRenewal(ctx, bearerToken(ctx), wallet.StaticRenewalRequest{IDs: req.GetAllocationIds(), Term: req.GetTerm()})
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	result := &v1.StaticRenewalQuote{ConfirmationToken: quote.ConfirmationToken, ExpiresAt: timestamppb.New(quote.ExpiresAt)}
	for _, line := range quote.Lines {
		result.Lines = append(result.Lines, &v1.StaticRenewalQuoteLine{AllocationId: line.ID, Ip: line.IP,
			UnitMinor: line.UnitMinor, TermDays: line.TermDays, OriginalExpiry: timestamppb.New(line.OriginalExpiry)})
	}
	return result, nil
}

func (s *Wallet) RenewStatic(ctx context.Context, req *v1.StaticRenewalRequest) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	order, err := s.biz.RenewStatic(ctx, bearerToken(ctx), wallet.StaticRenewalRequest{
		IDs: req.GetAllocationIds(), Term: req.GetTerm(), Key: req.GetRequestKey(), ConfirmationToken: req.GetConfirmationToken(),
	})
	return orderMessage(order), wallet.PublicError(err)
}

func (s *Wallet) RecheckStaticRenewal(ctx context.Context, req *v1.CustomerPurchaseOrderRequest) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	order, err := s.biz.RecheckStaticRenewal(ctx, bearerToken(ctx), req.GetOrderId())
	return orderMessage(order), wallet.PublicError(err)
}

func (s *Wallet) ResolveStaticRenewal(ctx context.Context, req *v1.StaticRenewalResolutionRequest) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	order, err := s.biz.ResolveStaticRenewal(ctx, bearerToken(ctx), wallet.StaticRenewalResolution{
		TenantID: req.GetTenantId(), OrderID: req.GetOrderId(), ItemID: req.GetAllocationId(), Evidence: req.GetEvidence(),
		CloseUnknown: req.GetCloseUnknown(), ConfirmedFailure: req.GetConfirmedFailure(),
	})
	return orderMessage(order), wallet.PublicError(err)
}
