package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Wallet) ExchangeStaticIP(ctx context.Context, req *v1.StaticIPExchangeRequest) (*v1.StaticIPExchange, error) {
	noStore(ctx)
	record, err := s.biz.ExchangeStaticIP(ctx, bearerToken(ctx), wallet.StaticExchangeRequest{
		AllocationID: req.GetAllocationId(), NodeID: req.GetNodeId(), IdempotencyKey: req.GetIdempotencyKey(),
	})
	return exchangeMessage(record, err)
}

func (s *Wallet) GetStaticIPExchange(ctx context.Context, req *v1.StaticIPExchangeLookup) (*v1.StaticIPExchange, error) {
	noStore(ctx)
	record, err := s.biz.StaticExchange(ctx, bearerToken(ctx), req.GetIdempotencyKey())
	return exchangeMessage(record, err)
}

func (s *Wallet) GetStaticIPExchangeQuota(ctx context.Context, _ *v1.BaseRequest) (*v1.StaticIPExchangeQuota, error) {
	noStore(ctx)
	quota, err := s.biz.StaticExchangeQuota(ctx, bearerToken(ctx))
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	return &v1.StaticIPExchangeQuota{Remaining: quota.Remaining, Reserved: quota.Reserved, Used: quota.Used}, nil
}

func exchangeMessage(record wallet.StaticExchange, err error) (*v1.StaticIPExchange, error) {
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	return &v1.StaticIPExchange{
		AllocationId: record.AllocationID, NodeId: record.NodeID, IdempotencyKey: record.IdempotencyKey,
		Status: record.Status, CreatedAt: timestamppb.New(record.CreatedAt), UpdatedAt: timestamppb.New(record.UpdatedAt),
	}, nil
}
