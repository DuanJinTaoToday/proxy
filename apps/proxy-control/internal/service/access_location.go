package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

type AccessLocation struct {
	biz     *access.LocationService
	proxies trustedProxies
}

func NewAccessLocation(biz *access.LocationService, server *conf.Server) (*AccessLocation, error) {
	proxies, err := parseTrustedProxies(server.GetHttp().GetTrustedProxyCidrs())
	if err != nil {
		return nil, err
	}
	return &AccessLocation{biz: biz, proxies: proxies}, nil
}

func (s *AccessLocation) GetAccessLocation(ctx context.Context, _ *v1.BaseRequest) (*v1.AccessLocation, error) {
	noStore(ctx)
	req, _ := kratoshttp.RequestFromServerContext(ctx)
	ip := s.proxies.sourceIP(req)
	location, err := s.biz.Locate(ctx, bearerToken(ctx), ip)
	if err != nil {
		return nil, err
	}
	response := &v1.AccessLocation{IsMainlandChina: location.IsMainlandChina}
	if ip.IsValid() {
		response.Ip = ip.String()
		response.Location = countryName(location, req.Header.Get("Accept-Language"))
	}
	return response, nil
}

var locationLanguage = language.NewMatcher([]language.Tag{language.English, language.SimplifiedChinese})

func countryName(location access.Location, acceptLanguage string) string {
	region, err := language.ParseRegion(location.CountryCode)
	if err != nil || !region.IsCountry() {
		return ""
	}
	_, index := language.MatchStrings(locationLanguage, acceptLanguage)
	switch {
	case index == 1 && location.IsMainlandChina:
		return "中国大陆"
	case location.IsMainlandChina:
		return "Mainland China"
	case index == 1:
		return display.SimplifiedChinese.Regions().Name(region)
	default:
		return display.English.Regions().Name(region)
	}
}
