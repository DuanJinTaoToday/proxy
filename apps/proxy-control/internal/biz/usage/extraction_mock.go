package usage

import (
	"context"
	"fmt"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

const MockExtractionSource = "api_extraction_mock"

type MockEndpoint struct {
	Host               string
	Port               uint32
	Username, Password string
	ExpiresAt          time.Time
}

func (s *Service) ExtractMock(ctx context.Context, token, tenantID string, count int32) ([]MockEndpoint, error) {
	if err := s.access.Require(token, tenantID, access.ExtractProxy); err != nil {
		return nil, err
	}
	if count < 1 || count > 100 {
		return nil, errors.BadRequest("INVALID_EXTRACTION_COUNT", "count must be between 1 and 100")
	}
	if err := s.requireMockTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)
	endpoints := make([]MockEndpoint, count)
	for i := range endpoints {
		endpoints[i] = MockEndpoint{
			Host: fmt.Sprintf("proxy-%d.example", i+1), Port: 1080,
			Username: "mock-" + tenantID, Password: "mock-password", ExpiresAt: expiresAt,
		}
	}
	return endpoints, nil
}

func (s *Service) requireMockTenant(ctx context.Context, tenantID string) error {
	enabled, err := s.repo.MockEnabled(ctx, tenantID)
	if err != nil {
		return err
	}
	if !enabled {
		return errors.NotFound("EXTRACTION_MOCK_UNAVAILABLE", "no mock usage has been imported for this tenant")
	}
	return nil
}
