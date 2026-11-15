package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/subaccount"
	"github.com/go-kratos/kratos/v3/errors"
)

type Subaccounts struct{ biz *subaccount.Service }

func NewSubaccounts(biz *subaccount.Service) *Subaccounts { return &Subaccounts{biz: biz} }

func subaccountMetadata(a subaccount.Account) *v1.Subaccount {
	return &v1.Subaccount{ClientId: a.ClientID, Name: a.Name, Remark: a.Remark, Active: a.Active, LimitBytes: a.LimitBytes, UsedBytes: a.UsedBytes}
}

func (s *Subaccounts) ListSubaccounts(ctx context.Context, _ *v1.BaseRequest) (*v1.Subaccounts, error) {
	noStore(ctx)
	rows, err := s.biz.List(ctx, bearerToken(ctx))
	if err != nil {
		return nil, err
	}
	result := &v1.Subaccounts{Accounts: make([]*v1.Subaccount, 0, len(rows))}
	for _, a := range rows {
		result.Accounts = append(result.Accounts, subaccountMetadata(a))
	}
	return result, nil
}

func (s *Subaccounts) CreateSubaccount(ctx context.Context, req *v1.CreateSubaccountRequest) (*v1.Subaccount, error) {
	noStore(ctx)
	a, err := s.biz.Create(ctx, bearerToken(ctx), req.GetName(), req.GetPassword(), req.GetRemark(), req.GetActive(), req.LimitBytes)
	if err != nil {
		return nil, err
	}
	return subaccountMetadata(a), nil
}

func (s *Subaccounts) UpdateSubaccount(ctx context.Context, req *v1.UpdateSubaccountRequest) (*v1.Subaccount, error) {
	noStore(ctx)
	u := subaccount.Update{}
	paths := req.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		return nil, errors.BadRequest("INVALID_UPDATE_MASK", "provide the fields to update")
	}
	for _, path := range paths {
		switch path {
		case "password":
			u.Password = &req.Password
		case "remark":
			u.Remark = &req.Remark
		case "active":
			u.Active = &req.Active
		case "limit_bytes":
			u.UpdateLimit, u.LimitBytes = true, req.LimitBytes
		default:
			return nil, errors.BadRequest("INVALID_UPDATE_MASK", "unsupported subaccount field")
		}
	}
	a, err := s.biz.Update(ctx, bearerToken(ctx), req.GetClientId(), u)
	if err != nil {
		return nil, err
	}
	return subaccountMetadata(a), nil
}

func (s *Subaccounts) RevealSubaccountPassword(ctx context.Context, req *v1.SubaccountRequest) (*v1.SubaccountPassword, error) {
	noStore(ctx)
	_, password, err := s.biz.Credentials(ctx, bearerToken(ctx), req.GetClientId(), false)
	if err != nil {
		return nil, err
	}
	return &v1.SubaccountPassword{Password: password}, nil
}

func (s *Subaccounts) GetProxyConfig(ctx context.Context, req *v1.SubaccountRequest) (*v1.SubaccountProxyConfig, error) {
	noStore(ctx)
	username, password, err := s.biz.Credentials(ctx, bearerToken(ctx), req.GetClientId(), true)
	if err != nil {
		return nil, err
	}
	return &v1.SubaccountProxyConfig{Username: username, Password: password, Host: s.biz.Host, Port: s.biz.Port, ExplicitSid: true}, nil
}

func authSnapshot(state subaccount.AuthState, hash []byte) *v1.AuthSnapshot {
	snapshot := &v1.AuthSnapshot{ClientId: state.ClientID, CredentialVersion: state.CredentialVersion, TenantId: state.TenantID, Username: state.Username, Reference: state.Reference, Active: state.Active, PasswordHash: hash}
	if state.Active {
		snapshot.Quota = &v1.AuthQuota{AvailableBytes: state.AvailableBytes}
	}
	return snapshot
}

func (s *Subaccounts) LoadAuthState(ctx context.Context, req *v1.LoadAuthStateRequest) (*v1.AuthSnapshot, error) {
	noStore(ctx)
	if req.Validate() != nil {
		return nil, errors.BadRequest("INVALID_AUTH_REQUEST", "invalid authentication request")
	}
	state, hash, err := s.biz.Load(ctx, bearerToken(ctx), req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return authSnapshot(state, hash), nil
}

func (s *Subaccounts) RefreshAuthState(ctx context.Context, req *v1.RefreshAuthStateRequest) (*v1.RefreshAuthStateResponse, error) {
	noStore(ctx)
	if req.Validate() != nil {
		return nil, errors.BadRequest("INVALID_AUTH_REQUEST", "invalid authentication request")
	}
	states, err := s.biz.Refresh(ctx, bearerToken(ctx), req.GetReferences())
	if err != nil {
		return nil, err
	}
	result := &v1.RefreshAuthStateResponse{Snapshots: make([]*v1.AuthSnapshot, 0, len(states))}
	for _, state := range states {
		hash, err := s.biz.Verifier(state)
		if err != nil {
			return nil, err
		}
		result.Snapshots = append(result.Snapshots, authSnapshot(state, hash))
	}
	return result, nil
}
