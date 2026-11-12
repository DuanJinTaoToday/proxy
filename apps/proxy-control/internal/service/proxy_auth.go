package service

import (
	"context"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProxyAuth struct {
	biz *proxyauth.Service
}

func NewProxyAuth(biz *proxyauth.Service) *ProxyAuth {
	return &ProxyAuth{biz: biz}
}

func (s *ProxyAuth) CreateCredential(ctx context.Context, req *v1.CreateCredentialRequest) (*v1.CreateCredentialResponse, error) {
	noStore(ctx)
	expiry, invalid := readTimestamp(req.GetExpiresAt())
	credential, err := s.biz.CreateCredential(ctx, bearerToken(ctx), req.GetTenantId(), expiry, invalid, req.GetAllowedSourceCidrs())
	if err != nil {
		return nil, err
	}
	return &v1.CreateCredentialResponse{ClientId: credential.ClientID, Username: credential.Username, Password: credential.Password}, nil
}

func (s *ProxyAuth) DisableCredential(ctx context.Context, req *v1.DisableCredentialRequest) (*v1.DisableCredentialResponse, error) {
	noStore(ctx)
	if err := s.biz.DisableCredential(ctx, bearerToken(ctx), req.GetTenantId(), req.GetClientId()); err != nil {
		return nil, err
	}
	return &v1.DisableCredentialResponse{}, nil
}

func (s *ProxyAuth) LoadAuthState(ctx context.Context, req *v1.LoadAuthStateRequest) (*v1.AuthSnapshot, error) {
	noStore(ctx)
	state, err := s.biz.LoadAuthState(ctx, bearerToken(ctx), req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return authSnapshot(state), nil
}

func (s *ProxyAuth) RefreshAuthState(ctx context.Context, req *v1.RefreshAuthStateRequest) (*v1.RefreshAuthStateResponse, error) {
	noStore(ctx)
	states, err := s.biz.RefreshAuthState(ctx, bearerToken(ctx), req.GetReferences())
	if err != nil {
		return nil, err
	}
	reply := &v1.RefreshAuthStateResponse{}
	for _, state := range states {
		reply.Snapshots = append(reply.Snapshots, authSnapshot(state))
	}
	return reply, nil
}

func authSnapshot(state *proxyauth.Snapshot) *v1.AuthSnapshot {
	reply := &v1.AuthSnapshot{
		Reference: state.Reference, Username: state.Username, ClientId: state.ClientID, TenantId: state.TenantID,
		PasswordHash: state.PasswordHash, Active: state.Active, ExpiresAt: timestamp(state.ExpiresAt),
		AllowedSourceCidrs: state.AllowedSourceCIDRs,
	}
	if q := state.Quota; q != nil {
		reply.Quota = &v1.AuthQuota{
			AvailableBytes: q.AvailableBytes, AdmissionThresholdBytes: q.AdmissionThresholdBytes, ValidUntil: timestamp(q.ValidUntil),
		}
	}
	return reply
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
