package service

import (
	"context"
	"math"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Usage) GetBillingSettings(ctx context.Context, req *v1.GetBillingSettingsRequest) (*v1.BillingSettingsResponse, error) {
	return s.billingSettings(ctx, req.GetScopeType(), req.GetScopeId(), req.GetTenantId(), nil)
}

func (s *Usage) UpdateBillingSettings(ctx context.Context, req *v1.UpdateBillingSettingsRequest) (*v1.BillingSettingsResponse, error) {
	if req.GetAdmissionThresholdBytes() > math.MaxInt64 || req.GetWarningThresholdBytes() > math.MaxInt64 {
		return nil, errors.BadRequest("INVALID_THRESHOLDS", "thresholds exceed the storage range")
	}
	settings := usage.Threshold{}
	if req.AdmissionThresholdBytes != nil {
		value := int64(req.GetAdmissionThresholdBytes())
		settings.AdmissionThresholdBytes = &value
	}
	if req.WarningThresholdBytes != nil {
		value := int64(req.GetWarningThresholdBytes())
		settings.WarningThresholdBytes = &value
	}
	return s.billingSettings(ctx, req.GetScopeType(), req.GetScopeId(), req.GetTenantId(), &settings)
}

func (s *Usage) billingSettings(ctx context.Context, scopeType, scopeID, tenant string, update *usage.Threshold) (*v1.BillingSettingsResponse, error) {
	noStore(ctx)
	settings, err := s.biz.BillingSettings(ctx, bearerToken(ctx), usage.Scope{Type: scopeType, ID: scopeID}, tenant, update)
	if err != nil {
		return nil, err
	}
	return &v1.BillingSettingsResponse{Configured: billingThresholds(&settings.Configured), Effective: billingThresholds(settings.Effective)}, nil
}

func billingThresholds(settings *usage.Threshold) *v1.BillingThresholds {
	if settings == nil {
		return nil
	}
	result := &v1.BillingThresholds{}
	if settings.AdmissionThresholdBytes != nil {
		value := uint64(*settings.AdmissionThresholdBytes)
		result.AdmissionThresholdBytes = &value
	}
	if settings.WarningThresholdBytes != nil {
		value := uint64(*settings.WarningThresholdBytes)
		result.WarningThresholdBytes = &value
	}
	return result
}

func (s *Usage) CreateBillingRate(ctx context.Context, req *v1.CreateBillingRateRequest) (*v1.BillingRateVersion, error) {
	noStore(ctx)
	at, err := grantTime(req.GetEffectiveFrom())
	if err != nil {
		return nil, errors.BadRequest("INVALID_EFFECTIVE_FROM", "invalid rate timestamp")
	}
	rate, err := s.biz.CreateBillingRate(ctx, bearerToken(ctx), usage.Scope{Type: req.GetScopeType(), ID: req.GetScopeId()},
		req.UploadRatioMilli, req.DownloadRatioMilli, at)
	if err != nil {
		return nil, err
	}
	return billingRateVersion(rate), nil
}

func (s *Usage) CreateTenantGroup(ctx context.Context, req *v1.CreateTenantGroupRequest) (*v1.TenantGroup, error) {
	noStore(ctx)
	if err := s.biz.CreateTenantGroup(ctx, bearerToken(ctx), req.GetGroupId(), req.GetName()); err != nil {
		return nil, err
	}
	return &v1.TenantGroup{GroupId: req.GetGroupId(), Name: req.GetName()}, nil
}

func (s *Usage) SetTenantGroup(ctx context.Context, req *v1.SetTenantGroupRequest) (*v1.TenantGroupMembership, error) {
	noStore(ctx)
	at, err := s.biz.SetTenantGroup(ctx, bearerToken(ctx), req.GetTenantId(), req.GetGroupId())
	if err != nil {
		return nil, err
	}
	return &v1.TenantGroupMembership{TenantId: req.GetTenantId(), GroupId: req.GetGroupId(), EffectiveFrom: timestamppb.New(at)}, nil
}
