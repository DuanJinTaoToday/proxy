package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Usage) ListDailyUsage(ctx context.Context, req *v1.DailyUsageRequest) (*v1.DailyUsageResponse, error) {
	noStore(ctx)
	rows, err := s.biz.DailyUsage(ctx, bearerToken(ctx), usage.DailyUsageFilter{
		ClientID: req.GetClientId(), StartDate: req.GetStartDate(), EndDate: req.GetEndDate(),
	})
	if err != nil {
		return nil, err
	}
	reply := &v1.DailyUsageResponse{Days: make([]*v1.DailyUsage, 0, len(rows))}
	for _, row := range rows {
		reply.Days = append(reply.Days, &v1.DailyUsage{
			Date: row.Date, BillableBytes: row.BillableBytes, UpdatedAt: timestamp(row.UpdatedAt),
		})
	}
	return reply, nil
}

func (s *Usage) GetTrafficAlert(ctx context.Context, _ *v1.BaseRequest) (*v1.TrafficAlert, error) {
	noStore(ctx)
	state, err := s.biz.TrafficAlert(ctx, bearerToken(ctx))
	if err != nil {
		return nil, err
	}
	return trafficAlertReply(state), nil
}

func (s *Usage) SetLowBalanceThreshold(ctx context.Context, req *v1.SetLowBalanceThresholdRequest) (*v1.TrafficAlert, error) {
	noStore(ctx)
	state, err := s.biz.SetTrafficAlert(ctx, bearerToken(ctx), req.ThresholdBytes)
	if err != nil {
		return nil, err
	}
	return trafficAlertReply(state), nil
}

func trafficAlertReply(state usage.TrafficAlert) *v1.TrafficAlert {
	return &v1.TrafficAlert{AvailableBytes: state.AvailableBytes, ThresholdBytes: state.ThresholdBytes,
		Occurrence: state.Occurrence, TriggeredAt: timestamp(state.TriggeredAt)}
}

func (s *Usage) ExportDailyUsage(ctx context.Context, req *v1.DailyUsageRequest) ([]byte, error) {
	reply, err := s.ListDailyUsage(ctx, req)
	if err != nil {
		return nil, err
	}
	records := [][]string{{"date", "billable_gb", "updated_at"}}
	for _, row := range reply.GetDays() {
		billable := row.GetBillableBytes()
		gb := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%d.%09d", billable/1_000_000_000, billable%1_000_000_000), "0"), ".")
		records = append(records, []string{row.GetDate(), gb, csvTimestamp(row.GetUpdatedAt())})
	}
	var buffer bytes.Buffer
	if err := csv.NewWriter(&buffer).WriteAll(records); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func csvTimestamp(value *timestamppb.Timestamp) string {
	if value == nil {
		return ""
	}
	return value.AsTime().Format(time.RFC3339Nano)
}
