package main

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
)

func openProxyAuth(settings *conf.ProxyAuth, accessService *access.Service) (*proxyauth.Service, func(), error) {
	repo, closeDB, err := data.OpenProxyAuth(settings)
	if err != nil {
		return nil, nil, err
	}
	return proxyauth.New(repo, accessService), closeDB, nil
}
