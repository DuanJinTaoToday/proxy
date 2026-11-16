package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/overview"
)

type Overview struct{ biz *overview.Service }

func NewOverview(biz *overview.Service) *Overview { return &Overview{biz: biz} }

func (s *Overview) GetOverview(ctx context.Context, _ *v1.BaseRequest) (*v1.Overview, error) {
	noStore(ctx)
	summary, err := s.biz.Get(ctx, bearerToken(ctx))
	if err != nil {
		return nil, err
	}
	reply := &v1.Overview{
		WalletMinor: summary.WalletMinor, TrafficBytes: summary.TrafficBytes,
		ActiveStaticIpCount: summary.Static.Active, CanCreateSubaccount: summary.CanCreateSubaccount,
	}
	if summary.LowTraffic {
		reply.Notices = append(reply.Notices, &v1.OverviewNotice{Kind: "low_traffic_balance", Count: 1})
	}
	if summary.Static.Expiring > 0 {
		reply.Notices = append(reply.Notices, &v1.OverviewNotice{Kind: "static_ip_expiry", Count: summary.Static.Expiring})
	}
	for _, item := range summary.Products {
		reply.Products = append(reply.Products, &v1.OverviewProduct{Kind: item.Kind, Open: item.Open, CanPurchase: item.CanPurchase})
	}
	return reply, nil
}
