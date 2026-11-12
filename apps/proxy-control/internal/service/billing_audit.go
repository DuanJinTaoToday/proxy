package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Usage) ListTrafficGrants(ctx context.Context, req *v1.ListTrafficGrantsRequest) (*v1.ListTrafficGrantsResponse, error) {
	noStore(ctx)
	page, err := s.biz.ListTrafficGrants(ctx, bearerToken(ctx), req.GetTenantId(), int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	reply := &v1.ListTrafficGrantsResponse{NextPageToken: page.NextToken, OverdraftBytes: uint64(page.OverdraftBytes)}
	for _, grant := range page.Items {
		reply.Grants = append(reply.Grants, &v1.AuditedTrafficGrant{
			GrantId: billingID(grant.ID), GrantedBytes: uint64(grant.GrantedBytes), UsedBytes: uint64(grant.UsedBytes),
			OverdraftOffsetBytes: uint64(grant.OverdraftOffsetBytes), StartsAt: timestamppb.New(grant.StartsAt),
			ExpiresAt: timestamp(grant.ExpiresAt), Status: grant.Status,
		})
	}
	return reply, nil
}

func (s *Usage) ListBalanceLedger(ctx context.Context, req *v1.ListBalanceLedgerRequest) (*v1.ListBalanceLedgerResponse, error) {
	noStore(ctx)
	page, err := s.biz.ListBalanceLedger(ctx, bearerToken(ctx), req.GetTenantId(), int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	reply := &v1.ListBalanceLedgerResponse{NextPageToken: page.NextToken}
	for _, entry := range page.Items {
		reply.Entries = append(reply.Entries, &v1.BalanceLedgerEntry{
			LedgerId: billingID(entry.ID), Kind: entry.Kind, GrantId: billingID(entry.GrantID), DeltaBytes: entry.DeltaBytes,
			AvailableBefore: entry.AvailableBefore, AvailableAfter: entry.AvailableAfter,
			ExternalRef: entry.ExternalRef, Reason: entry.Reason, Operator: entry.Operator,
			CreatedAt: timestamppb.New(entry.CreatedAt), CorrectionOf: billingID(entry.CorrectionOf),
		})
	}
	return reply, nil
}

func (s *Usage) ListBillingRates(ctx context.Context, req *v1.ListBillingRatesRequest) (*v1.ListBillingRatesResponse, error) {
	noStore(ctx)
	page, err := s.biz.ListBillingRates(ctx, bearerToken(ctx), req.GetScopeType(), req.GetScopeId(), int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	reply := &v1.ListBillingRatesResponse{NextPageToken: page.NextToken}
	for _, rate := range page.Items {
		reply.Rates = append(reply.Rates, &v1.BillingRateVersion{
			RateId: billingID(rate.ID), ScopeType: rate.ScopeType, ScopeId: rate.ScopeID,
			UploadRatioMilli: uint64(rate.UploadRatioMilli), DownloadRatioMilli: uint64(rate.DownloadRatioMilli),
			EffectiveFrom: timestamppb.New(rate.EffectiveFrom),
		})
	}
	return reply, nil
}
