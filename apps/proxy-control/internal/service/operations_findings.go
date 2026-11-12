package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Usage) ListRecordAnomalies(ctx context.Context, req *v1.ListRecordAnomaliesRequest) (*v1.ListRecordAnomaliesResponse, error) {
	noStore(ctx)
	filter := usage.FindingFilter{
		GatewayID: req.GetGatewayId(), BootID: req.GetBootId(), TenantID: req.GetTenantId(), Kind: req.GetKind(),
	}
	page, err := s.biz.ListRecordAnomalies(ctx, bearerToken(ctx), filter, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	reply := &v1.ListRecordAnomaliesResponse{NextPageToken: page.NextToken}
	for _, row := range page.Items {
		reply.Anomalies = append(reply.Anomalies, &v1.RecordAnomaly{
			AnomalyId: optionalNumericID(row.ID), GatewayId: row.GatewayID, BootId: row.BootID, TenantId: row.TenantID,
			Kind: row.Kind, FileId: row.FileID, FileOffset: row.FileOffset,
			DetectedAt: timestamppb.New(row.DetectedAt), ResolvedAt: timestamp(row.ResolvedAt),
		})
	}
	return reply, nil
}

func (s *Usage) ListUsageGaps(ctx context.Context, req *v1.ListUsageGapsRequest) (*v1.ListUsageGapsResponse, error) {
	noStore(ctx)
	filter := usage.FindingFilter{
		GatewayID: req.GetGatewayId(), BootID: req.GetBootId(), TenantID: req.GetTenantId(),
		Kind: req.GetKind(), Status: req.GetStatus(),
	}
	page, err := s.biz.ListUsageGaps(ctx, bearerToken(ctx), filter, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, err
	}
	reply := &v1.ListUsageGapsResponse{NextPageToken: page.NextToken}
	for _, row := range page.Items {
		reply.Gaps = append(reply.Gaps, &v1.UsageGap{
			GapId: optionalNumericID(row.ID), GatewayId: row.GatewayID, BootId: row.BootID, TenantId: row.TenantID,
			Kind: row.Kind, Status: row.Status, SessionId: row.SessionID, FromAt: timestamppb.New(row.FromAt),
			DetectedAt: timestamppb.New(row.DetectedAt), ResolvedAt: timestamp(row.ResolvedAt),
			ExitedAt: timestamp(row.ExitedAt),
		})
	}
	return reply, nil
}
