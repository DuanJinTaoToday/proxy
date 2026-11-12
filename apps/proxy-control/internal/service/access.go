package service

import (
	"context"
	"strings"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/transport"
)

type Access struct {
	biz *access.Service
}

func NewAccess(biz *access.Service) *Access {
	return &Access{biz: biz}
}

func (s *Access) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	noStore(ctx)
	token, expires, err := s.biz.Login(req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return &v1.LoginResponse{AccessToken: token, ExpiresInSeconds: expires}, nil
}

func (s *Access) ListTenants(ctx context.Context, _ *v1.ListTenantsRequest) (*v1.ListTenantsResponse, error) {
	noStore(ctx)
	tenants, err := s.biz.ListTenants(bearerToken(ctx))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListTenantsResponse{}
	for _, tenant := range tenants {
		reply.Tenants = append(reply.Tenants, tenantAccess(tenant))
	}
	return reply, nil
}

func (s *Access) GetTenantAccess(ctx context.Context, req *v1.GetTenantAccessRequest) (*v1.TenantAccess, error) {
	noStore(ctx)
	tenant, err := s.biz.GetTenantAccess(bearerToken(ctx), req.GetTenantId())
	if err != nil {
		return nil, err
	}
	return tenantAccess(tenant), nil
}

func (s *Access) CheckPermission(ctx context.Context, req *v1.CheckPermissionRequest) (*v1.CheckPermissionResponse, error) {
	noStore(ctx)
	var permission access.Permission
	switch req.GetPermission() {
	case v1.Permission_PERMISSION_READ_TENANT_USAGE:
		permission = access.ReadTenantUsage
	case v1.Permission_PERMISSION_MANAGE_PROXY_CREDENTIALS:
		permission = access.ManageProxyCredentials
	case v1.Permission_PERMISSION_READ_OPERATIONS:
		permission = access.ReadOperations
	case v1.Permission_PERMISSION_EXTRACT_PROXY:
		permission = access.ExtractProxy
	case v1.Permission_PERMISSION_MANAGE_BILLING:
		permission = access.ManageBilling
	}
	if err := s.biz.Require(bearerToken(ctx), req.GetTenantId(), permission); err != nil {
		return nil, err
	}
	return &v1.CheckPermissionResponse{}, nil
}

func tenantAccess(tenant access.Tenant) *v1.TenantAccess {
	return &v1.TenantAccess{TenantId: tenant.ID, Name: tenant.Name}
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
