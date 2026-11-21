package service

import (
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
)

func TestAccessLocationCountryNames(t *testing.T) {
	tests := []struct {
		location access.Location
		language string
		want     string
	}{
		{access.Location{CountryCode: "CN", IsMainlandChina: true}, "zh-CN", "中国大陆"},
		{access.Location{CountryCode: "CN", IsMainlandChina: true}, "en", "Mainland China"},
		{access.Location{CountryCode: "US"}, "zh-CN", "美国"},
		{access.Location{CountryCode: "US"}, "", "United States"},
		{access.Location{CountryCode: "ZZ"}, "zh-CN", ""},
		{access.Location{}, "en", ""},
	}
	for _, tt := range tests {
		if got := countryName(tt.location, tt.language); got != tt.want {
			t.Errorf("country name for %q in %q = %q, want %q", tt.location.CountryCode, tt.language, got, tt.want)
		}
	}
}
