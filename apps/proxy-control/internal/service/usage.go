package service

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"time"

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
	response := &v1.SubmitBatchResponse{
		Billable: result.Billable, NotBillable: result.NotBillable, Quarantined: result.Quarantined, Duplicates: result.Duplicates,
	}
	for _, file := range slices.Sorted(maps.Keys(result.ConfirmedOffsets)) {
		response.Watermarks = append(response.Watermarks, &v1.RecorderWatermark{FileId: file, ConfirmedOffset: result.ConfirmedOffsets[file]})
	}
	return response, nil
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

func (s *Usage) QueryUsage(ctx context.Context, req *v1.QueryUsageRequest) (*v1.QueryUsageResponse, error) {
	noStore(ctx)
	start, _ := readTimestamp(req.GetStartTime())
	end, _ := readTimestamp(req.GetEndTime())
	query := usage.UsageQuery{
		TenantID: req.GetTenantId(), GatewayID: req.GetGatewayId(), ClientID: req.GetClientId(), SplitByGateway: req.GetSplitByGateway(),
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

func readTimestamp(value *timestamppb.Timestamp) (at *time.Time, invalid bool) {
	if value == nil {
		return nil, false
	}
	if !value.IsValid() {
		return nil, true
	}
	t := value.AsTime()
	return &t, false
}

func timestamp(at *time.Time) *timestamppb.Timestamp {
	if at == nil {
		return nil
	}
	return timestamppb.New(*at)
}
