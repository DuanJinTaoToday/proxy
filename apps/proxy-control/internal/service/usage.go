package service

import (
	"context"
	"strconv"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Usage struct {
	biz *usage.Service
}

func NewUsage(biz *usage.Service) *Usage {
	return &Usage{biz: biz}
}

func (s *Usage) SubmitBatch(ctx context.Context, req *v1.SubmitBatchRequest) (*v1.SubmitBatchResponse, error) {
	batch := usage.Batch{GatewayID: req.GetGatewayId(), BootID: req.GetBootId(), Records: make([]usage.RecorderLine, 0, len(req.GetRecords()))}
	for _, line := range req.GetRecords() {
		batch.Records = append(batch.Records, usage.RecorderLine{
			FileID: line.GetFileId(), FileOffset: line.GetFileOffset(), EndOffset: line.GetEndOffset(), Line: line.GetLine(),
		})
	}
	result, err := s.biz.SubmitBatch(ctx, bearerToken(ctx), batch)
	if err != nil {
		return nil, err
	}
	return &v1.SubmitBatchResponse{
		Billable: result.Billable, NotBillable: result.NotBillable, Quarantined: result.Quarantined, Duplicates: result.Duplicates,
	}, nil
}

func (s *Usage) ReportLifecycle(ctx context.Context, req *v1.ReportLifecycleRequest) (*v1.ReportLifecycleResponse, error) {
	exit, invalid := readTimestamp(req.GetExitedAt())
	if err := s.biz.ReportLifecycle(ctx, bearerToken(ctx), req.GetGatewayId(), req.GetBootId(), exit, invalid); err != nil {
		return nil, err
	}
	return &v1.ReportLifecycleResponse{}, nil
}

func (s *Usage) GetTenantDailyUsage(ctx context.Context, req *v1.DailyUsageRequest) (*v1.DailyUsageResponse, error) {
	noStore(ctx)
	result, err := s.biz.GetTenantDailyUsage(ctx, bearerToken(ctx), req.GetTenantId(), req.GetStartDate(), req.GetEndDate())
	if err != nil {
		return nil, err
	}
	reply := &v1.DailyUsageResponse{LastReceivedAt: timestamp(result.LastReceivedAt)}
	for _, day := range result.Days {
		reply.Days = append(reply.Days, &v1.DailyUsage{Date: day.Date, Amount: &v1.UsageAmount{
			InputBytes: day.InputBytes, OutputBytes: day.OutputBytes, TotalBytes: day.TotalBytes, BillableBytes: day.BillableBytes,
		}})
	}
	return reply, nil
}

func (s *Usage) GetTenantBalance(ctx context.Context, req *v1.TenantBalanceRequest) (*v1.TenantBalance, error) {
	noStore(ctx)
	balance, err := s.biz.GetTenantBalance(ctx, bearerToken(ctx), req.GetTenantId())
	if err != nil {
		return nil, err
	}
	reply := &v1.TenantBalance{
		TenantId: balance.TenantID, AvailableBytes: balance.AvailableBytes, OverdraftBytes: balance.OverdraftBytes,
		AdmissionThresholdBytes: balance.AdmissionThresholdBytes, WarningThresholdBytes: balance.WarningThresholdBytes,
		WarningSince: timestamp(balance.WarningSince),
	}
	for _, grant := range balance.Grants {
		reply.Grants = append(reply.Grants, &v1.TrafficGrant{
			GrantId: strconv.FormatInt(grant.ID, 10), GrantedBytes: uint64(grant.GrantedBytes), UsedBytes: uint64(grant.UsedBytes),
			StartsAt: timestamppb.New(grant.StartsAt), ExpiresAt: timestamp(grant.ExpiresAt), Source: grant.Source,
		})
	}
	return reply, nil
}
