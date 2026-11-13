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

func (s *Usage) ReportLifecycle(ctx context.Context, req *v1.ReportLifecycleRequest) (*v1.BaseResp, error) {
	exit, invalid := readTimestamp(req.GetExitedAt())
	if err := s.biz.ReportLifecycle(ctx, bearerToken(ctx), req.GetGatewayId(), req.GetBootId(), exit, invalid); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}

func (s *Usage) ReportUnfinishedSessions(ctx context.Context, req *v1.ReportUnfinishedSessionsRequest) (*v1.BaseResp, error) {
	sessions := make([]usage.UnfinishedSession, 0, len(req.GetSessions()))
	for _, session := range req.GetSessions() {
		recordedAt, invalid := readTimestamp(session.GetLastRecordedAt())
		item := usage.UnfinishedSession{SessionID: session.GetSessionId(), ClientID: session.GetClientId()}
		if !invalid && recordedAt != nil {
			item.LastRecordedAt = *recordedAt
		}
		sessions = append(sessions, item)
	}
	if err := s.biz.ReportUnfinishedSessions(ctx, bearerToken(ctx), req.GetGatewayId(), req.GetBootId(), sessions); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}

func (s *Usage) CompleteReplay(ctx context.Context, req *v1.CompleteReplayRequest) (*v1.BaseResp, error) {
	if err := s.biz.CompleteReplay(ctx, bearerToken(ctx), req.GetGatewayId(), req.GetBootId(), req.GetUnfinishedSessionCount(), req.HasSessionRecords); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}

func (s *Usage) ExtractProxy(ctx context.Context, req *v1.ExtractProxyRequest) (*v1.ExtractProxyResponse, error) {
	noStore(ctx)
	endpoints, err := s.biz.ExtractMock(ctx, bearerToken(ctx), req.GetTenantId(), req.GetCount())
	if err != nil {
		return nil, err
	}
	reply := &v1.ExtractProxyResponse{}
	for _, endpoint := range endpoints {
		reply.IpList = append(reply.IpList, &v1.ProxyEndpoint{
			Host: endpoint.Host, Port: endpoint.Port, Username: endpoint.Username,
			Password: endpoint.Password, ExpiresAt: timestamppb.New(endpoint.ExpiresAt),
		})
	}
	return reply, nil
}

func (s *Usage) GetTenantDailyUsage(ctx context.Context, req *v1.DailyUsageRequest) (*v1.DailyUsageResponse, error) {
	noStore(ctx)
	result, err := s.biz.GetTenantDailyUsage(ctx, bearerToken(ctx), req.GetTenantId(), req.GetStartDate(), req.GetEndDate(), req.GetSource())
	if err != nil {
		return nil, err
	}
	return dailyUsageResponse(result), nil
}

func (s *Usage) QueryUsage(ctx context.Context, req *v1.QueryUsageRequest) (*v1.QueryUsageResponse, error) {
	noStore(ctx)
	start, _ := readTimestamp(req.GetStartTime())
	end, _ := readTimestamp(req.GetEndTime())
	query := usage.UsageQuery{
		TenantID: req.GetTenantId(), GatewayID: req.GetGatewayId(), SplitByGateway: req.GetSplitByGateway(),
		SplitByGrant: req.GetSplitByGrant(), SplitByRate: req.GetSplitByRate(),
	}
	if start != nil && end != nil {
		query.Start, query.End = *start, *end
	}
	switch req.GetGranularity() {
	case v1.UsageGranularity_USAGE_GRANULARITY_MINUTE:
		query.Granularity = usage.Minute
	case v1.UsageGranularity_USAGE_GRANULARITY_HOUR:
		query.Granularity = usage.Hour
	case v1.UsageGranularity_USAGE_GRANULARITY_DAY:
		query.Granularity = usage.DayGranularity
	}
	result, err := s.biz.QueryUsage(ctx, bearerToken(ctx), query, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	reply := &v1.QueryUsageResponse{NextPageToken: result.NextToken}
	for _, bucket := range result.Buckets {
		reply.Buckets = append(reply.Buckets, &v1.UsageBucket{
			StartTime: timestamppb.New(bucket.StartTime),
			GatewayId: bucket.GatewayID,
			GrantId:   optionalNumericID(bucket.GrantID),
			RateId:    optionalNumericID(bucket.RateID),
			Amount: &v1.UsageAmount{
				InputBytes: bucket.InputBytes, OutputBytes: bucket.OutputBytes,
				TotalBytes: bucket.InputBytes + bucket.OutputBytes, BillableBytes: bucket.BillableBytes,
			},
		})
	}
	return reply, nil
}

func optionalNumericID(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

func dailyUsageResponse(result *usage.DailyUsage) *v1.DailyUsageResponse {
	reply := &v1.DailyUsageResponse{LastReceivedAt: timestamp(result.LastReceivedAt)}
	for _, day := range result.Days {
		reply.Days = append(reply.Days, &v1.DailyUsage{Date: day.Date, Amount: dailyAmount(day)})
	}
	return reply
}

func dailyAmount(day usage.Day) *v1.UsageAmount {
	return &v1.UsageAmount{
		InputBytes: day.InputBytes, OutputBytes: day.OutputBytes, TotalBytes: day.TotalBytes, BillableBytes: day.BillableBytes,
	}
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
