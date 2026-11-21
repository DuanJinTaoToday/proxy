package access

import (
	"context"
	"net/netip"
)

type IPCountryLookup interface {
	CountryCode(netip.Addr) string
}

type LocationService struct {
	access    *Service
	countries IPCountryLookup
}

func NewLocationService(access *Service, countries IPCountryLookup) *LocationService {
	return &LocationService{access: access, countries: countries}
}

type Location struct {
	CountryCode     string
	IsMainlandChina bool
}

func (s *LocationService) Locate(ctx context.Context, token string, ip netip.Addr) (Location, error) {
	if _, err := s.access.Account(ctx, token); err != nil {
		return Location{}, err
	}
	countryCode := s.countries.CountryCode(ip)
	return Location{CountryCode: countryCode, IsMainlandChina: countryCode == "CN"}, nil
}
