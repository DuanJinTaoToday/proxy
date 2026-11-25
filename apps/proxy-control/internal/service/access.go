package service

import (
	"context"
	"net/netip"
	"strings"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/go-kratos/kratos/v3/transport"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type Access struct {
	biz     *access.Service
	proxies trustedProxies
}

func NewAccess(biz *access.Service, server *conf.Server) (*Access, error) {
	proxies, err := parseTrustedProxies(server.GetHttp().GetTrustedProxyCidrs())
	if err != nil {
		return nil, err
	}
	return &Access{biz: biz, proxies: proxies}, nil
}

func (s *Access) SendRegistrationCode(ctx context.Context, req *v1.EmailRequest) (*v1.BaseResp, error) {
	return s.requestEmail(ctx, req.GetEmail(), access.RegistrationPurpose)
}

func (s *Access) ForgotPassword(ctx context.Context, req *v1.EmailRequest) (*v1.BaseResp, error) {
	return s.requestEmail(ctx, req.GetEmail(), access.PasswordResetPurpose)
}

func (s *Access) requestEmail(ctx context.Context, email, purpose string) (*v1.BaseResp, error) {
	noStore(ctx)
	req, _ := kratoshttp.RequestFromServerContext(ctx)
	ip := s.proxies.sourceIP(req)
	source := "unknown"
	switch {
	case ip.Is4():
		source = ip.String()
	case ip.Is6():
		source = netip.PrefixFrom(ip, 64).Masked().String()
	}
	if err := s.biz.RequestEmail(ctx, email, source, purpose); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}

func (s *Access) ResetCustomerPassword(ctx context.Context, req *v1.EmailCredentialRequest) (*v1.BaseResp, error) {
	return s.redeemEmailLink(ctx, req, access.PasswordResetPurpose)
}

func (s *Access) RecoverCustomerEmail(ctx context.Context, req *v1.EmailCredentialRequest) (*v1.BaseResp, error) {
	return s.redeemEmailLink(ctx, req, access.RecoveryPurpose)
}

func (s *Access) redeemEmailLink(ctx context.Context, req *v1.EmailCredentialRequest, purpose string) (*v1.BaseResp, error) {
	noStore(ctx)
	if err := s.biz.RedeemEmailLink(ctx, purpose, req.GetToken(), req.GetPassword()); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}

func (s *Access) StartEmailRecovery(ctx context.Context, req *v1.StartEmailRecoveryRequest) (*v1.BaseResp, error) {
	noStore(ctx)
	if err := s.biz.StartEmailRecovery(ctx, bearerToken(ctx), req.GetTenantId(), req.GetEmail()); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}

func (s *Access) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	noStore(ctx)
	token, expires, err := s.biz.LoginCustomer(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return &v1.LoginResponse{AccessToken: token, ExpiresInSeconds: expires}, nil
}

func (s *Access) CheckPermission(ctx context.Context, req *v1.CheckPermissionRequest) (*v1.BaseResp, error) {
	noStore(ctx)
	var permission access.Permission
	switch req.GetPermission() {
	case v1.Permission_PERMISSION_READ_TENANT_USAGE:
		permission = access.ReadTenantUsage
	case v1.Permission_PERMISSION_READ_OPERATIONS:
		permission = access.ReadOperations
	case v1.Permission_PERMISSION_MANAGE_BILLING:
		permission = access.ManageBilling
	}
	if err := s.biz.Require(bearerToken(ctx), req.GetTenantId(), permission); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}

func (s *Access) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.TenantAccess, error) {
	noStore(ctx)
	tenant, err := s.biz.Register(ctx, req.GetEmail(), req.GetPassword(), req.GetVerificationCode())
	if err != nil {
		return nil, err
	}
	return tenantAccess(tenant), nil
}

func (s *Access) GetAccount(ctx context.Context, _ *v1.BaseRequest) (*v1.TenantAccess, error) {
	noStore(ctx)
	tenant, err := s.biz.Account(ctx, bearerToken(ctx))
	if err != nil {
		return nil, err
	}
	return tenantAccess(tenant), nil
}

func (s *Access) LoginOperator(ctx context.Context, req *v1.LoginOperatorRequest) (*v1.LoginResponse, error) {
	noStore(ctx)
	token, expires, err := s.biz.LoginOperator(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return &v1.LoginResponse{AccessToken: token, ExpiresInSeconds: expires}, nil
}

func (s *Access) GetOperatorAccount(ctx context.Context, _ *v1.BaseRequest) (*v1.OperatorAccount, error) {
	noStore(ctx)
	username, permissions, err := s.biz.OperatorAccount(ctx, bearerToken(ctx))
	if err != nil {
		return nil, err
	}
	return &v1.OperatorAccount{Username: username, Permissions: permissions}, nil
}

func (s *Access) ResetTenantPassword(ctx context.Context, req *v1.ResetTenantPasswordRequest) (*v1.BaseResp, error) {
	noStore(ctx)
	if err := s.biz.ResetTenantPassword(ctx, bearerToken(ctx), req.GetTenantId(), req.GetPassword()); err != nil {
		return nil, err
	}
	return &v1.BaseResp{}, nil
}

func tenantAccess(tenant access.Tenant) *v1.TenantAccess {
	return &v1.TenantAccess{TenantId: tenant.ID, Name: tenant.Name, PrimaryAccountName: tenant.PrimaryAccountName, Email: tenant.Email}
}

func noStore(ctx context.Context) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Cache-Control", "no-store")
	}
}

func bearerToken(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		scheme, token, _ := strings.Cut(tr.RequestHeader().Get("Authorization"), " ")
		if strings.EqualFold(scheme, "Bearer") && len(token) <= 256 {
			return token
		}
	}
	return ""
}
