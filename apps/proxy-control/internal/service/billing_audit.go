package service

import (
	"context"
	"strconv"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Usage) CreateRecharge(ctx context.Context, req *v1.CreateRechargeRequest) (*v1.BalanceLedgerEntry, error) {
	noStore(ctx)
	starts, err := grantTime(req.GetStartsAt())
	if err != nil {
		return nil, err
	}
	expires, err := grantTime(req.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	entry, err := s.biz.ChangeBalance(ctx, bearerToken(ctx), req.GetTenantId(), usage.BalanceChange{
		Kind: "recharge", ExternalRef: req.GetExternalRef(), Bytes: req.GetGrantedBytes(),
		StartsAt: starts, ExpiresAt: expires, Reason: req.GetNote(),
	})
	if err != nil {
		return nil, err
	}
	return balanceLedgerEntry(entry), nil
}

func (s *Usage) AdjustBalance(ctx context.Context, req *v1.AdjustBalanceRequest) (*v1.BalanceLedgerEntry, error) {
	noStore(ctx)
	change := usage.BalanceChange{ExternalRef: req.GetExternalRef(), Bytes: req.GetBytes(), Reason: req.GetReason()}
	switch req.GetMode() {
	case v1.BalanceAdjustmentMode_ADD:
		change.Kind = "adjust_add"
	case v1.BalanceAdjustmentMode_SUBTRACT:
		change.Kind = "adjust_subtract"
	}
	if req.GetGrantId() != "" {
		id, err := strconv.ParseInt(req.GetGrantId(), 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.BadRequest("INVALID_GRANT_ID", "grant_id must be a positive number")
		}
		change.GrantID = id
	}
	entry, err := s.biz.ChangeBalance(ctx, bearerToken(ctx), req.GetTenantId(), change)
	if err != nil {
		return nil, err
	}
	return balanceLedgerEntry(entry), nil
}

func grantTime(value *timestamppb.Timestamp) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, errors.BadRequest("INVALID_GRANT_WINDOW", "invalid grant timestamp")
	}
	at := value.AsTime()
	return &at, nil
}

func (s *Usage) ListTrafficGrants(ctx context.Context, req *v1.ListTrafficGrantsRequest) (*v1.ListTrafficGrantsResponse, error) {
	noStore(ctx)
	page, err := s.biz.ListTrafficGrants(ctx, bearerToken(ctx), req.GetTenantId(), int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	reply := &v1.ListTrafficGrantsResponse{NextPageToken: page.NextToken, OverdraftBytes: uint64(page.OverdraftBytes)}
	for _, grant := range page.Items {
		reply.Grants = append(reply.Grants, &v1.AuditedTrafficGrant{
			GrantId: optionalNumericID(grant.ID), GrantedBytes: uint64(grant.GrantedBytes), UsedBytes: uint64(grant.UsedBytes),
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
		reply.Entries = append(reply.Entries, balanceLedgerEntry(entry))
	}
	return reply, nil
}

func balanceLedgerEntry(entry usage.LedgerEntry) *v1.BalanceLedgerEntry {
	return &v1.BalanceLedgerEntry{
		LedgerId: optionalNumericID(entry.ID), Kind: entry.Kind, GrantId: optionalNumericID(entry.GrantID), DeltaBytes: entry.DeltaBytes,
		AvailableBefore: entry.AvailableBefore, AvailableAfter: entry.AvailableAfter,
		ExternalRef: entry.ExternalRef, Reason: entry.Reason, Operator: entry.Operator,
		CreatedAt: timestamppb.New(entry.CreatedAt), CorrectionOf: optionalNumericID(entry.CorrectionOf),
		OverdraftAddedBytes: entry.OverdraftAddedBytes,
	}
}

func (s *Usage) ListBillingRates(ctx context.Context, req *v1.ListBillingRatesRequest) (*v1.ListBillingRatesResponse, error) {
	noStore(ctx)
	page, err := s.biz.ListBillingRates(ctx, bearerToken(ctx), req.GetScopeType(), req.GetScopeId(), req.GetRateId(), int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	reply := &v1.ListBillingRatesResponse{NextPageToken: page.NextToken}
	for _, rate := range page.Items {
		reply.Rates = append(reply.Rates, billingRateVersion(rate))
	}
	return reply, nil
}

func billingRateVersion(rate usage.RateVersion) *v1.BillingRateVersion {
	return &v1.BillingRateVersion{
		RateId: optionalNumericID(rate.ID), ScopeType: rate.ScopeType, ScopeId: rate.ScopeID,
		UploadRatioMilli: uint64(rate.UploadRatioMilli), DownloadRatioMilli: uint64(rate.DownloadRatioMilli),
		EffectiveFrom: timestamppb.New(rate.EffectiveFrom),
	}
}
