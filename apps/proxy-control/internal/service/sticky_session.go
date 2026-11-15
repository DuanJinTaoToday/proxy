package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/subaccount"
)

func (s *Subaccounts) AcquireStickySession(ctx context.Context, req *v1.StickySessionRequest) (*v1.StickySession, error) {
	noStore(ctx)
	a, err := s.biz.Acquire(ctx, bearerToken(ctx), subaccount.SessionRequest{Reference: req.GetReference(), Key: req.GetSessionKey(), Attempt: req.GetRequestKey()}, req.Validate)
	if err != nil {
		return nil, err
	}
	return &v1.StickySession{UpstreamSid: a.UpstreamSID, ExpiresInMillis: a.ExpiresInMillis, LeaseToken: a.LeaseToken}, nil
}

func (s *Subaccounts) CommitStickySession(ctx context.Context, req *v1.StickySessionRequest) (*v1.StickySession, error) {
	noStore(ctx)
	a, err := s.biz.Commit(ctx, bearerToken(ctx), subaccount.SessionRequest{Reference: req.GetReference(), Key: req.GetSessionKey(), Attempt: req.GetLeaseToken()}, req.Validate)
	if err != nil {
		return nil, err
	}
	return &v1.StickySession{UpstreamSid: a.UpstreamSID, ExpiresInMillis: a.ExpiresInMillis}, nil
}

func (s *Subaccounts) ReleaseStickySession(ctx context.Context, req *v1.StickySessionRequest) (*v1.BaseResp, error) {
	noStore(ctx)
	if err := s.biz.Release(ctx, bearerToken(ctx), subaccount.SessionRequest{Reference: req.GetReference(), Key: req.GetSessionKey(), Attempt: req.GetLeaseToken()}, req.Validate); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}
