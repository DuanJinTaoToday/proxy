package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

func (s *Subaccounts) AcquireStickySession(ctx context.Context, req *v1.StickySessionRequest) (*v1.StickySession, error) {
	noStore(ctx)
	a, err := s.biz.Acquire(ctx, bearerToken(ctx), req.GetClientId(), req.GetSessionKey())
	if err != nil {
		return nil, err
	}
	return &v1.StickySession{UpstreamSid: a.UpstreamSID, LeaseToken: a.LeaseToken}, nil
}

func (s *Subaccounts) CommitStickySession(ctx context.Context, req *v1.StickySessionRequest) (*v1.StickySession, error) {
	noStore(ctx)
	a, err := s.biz.Commit(ctx, bearerToken(ctx), req.GetClientId(), req.GetSessionKey(), req.GetLeaseToken())
	if err != nil {
		return nil, err
	}
	return &v1.StickySession{UpstreamSid: a.UpstreamSID}, nil
}

func (s *Subaccounts) ReleaseStickySession(ctx context.Context, req *v1.StickySessionRequest) (*v1.BaseResp, error) {
	noStore(ctx)
	if err := s.biz.Release(ctx, bearerToken(ctx), req.GetClientId(), req.GetSessionKey(), req.GetLeaseToken()); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}
