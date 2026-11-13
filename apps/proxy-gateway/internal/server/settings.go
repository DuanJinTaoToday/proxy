package server

import (
	"encoding/base64"
	"errors"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/acl"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/gost"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
)

type GatewaySettings struct {
	binary, template string
	Spool            repo.SpoolPath
	LocalToken       string
	wukong           gost.WukongConfig
	Address          netip.AddrPort
	Recorder         recorder.Config
	Auth             acl.AuthConfig
	Report           acl.ReportConfig
}

func NewGatewaySettings(server *conf.Server, cfg *conf.Data, biz *conf.Biz) (GatewaySettings, error) {
	gostCfg := cfg.GetGost()
	controlCfg := cfg.GetAcl().GetControl()
	recorderCfg := biz.GetRecorder()
	recorderSettings := recorder.Config{GatewayID: biz.GetGatewayId(), MaxRecords: 1000, MaxBytes: 1 << 20, Interval: time.Second}
	reportTimeout := 3 * time.Second
	if recorderCfg != nil && recorderCfg.MaxRecords != nil {
		recorderSettings.MaxRecords = int(recorderCfg.GetMaxRecords())
	}
	if recorderCfg != nil && recorderCfg.MaxBytes != nil {
		recorderSettings.MaxBytes = int(recorderCfg.GetMaxBytes())
	}
	if duration := recorderCfg.GetScanInterval(); duration != nil {
		recorderSettings.Interval = duration.AsDuration()
	}
	if duration := recorderCfg.GetReportTimeout(); duration != nil {
		reportTimeout = duration.AsDuration()
	}
	spoolPath := cfg.GetRepo().GetRecorder().GetSpool()
	if spoolPath == "" || gostCfg.GetBinary() == "" || gostCfg.GetConfig() == "" ||
		recorderSettings.GatewayID == "" || len(recorderSettings.GatewayID) > 128 ||
		recorderSettings.MaxRecords < 1 || recorderSettings.MaxBytes < 128 || recorderSettings.MaxBytes > 16<<20 ||
		recorderSettings.Interval <= 0 || reportTimeout <= 0 {
		return GatewaySettings{}, errors.New("configure GOST binary and template, Recorder spool, gateway ID and positive report limits")
	}
	spool, err := filepath.Abs(spoolPath)
	if err != nil {
		return GatewaySettings{}, err
	}
	if err := os.MkdirAll(spool, 0o700); err != nil {
		return GatewaySettings{}, err
	}
	address := server.GetHttp().GetAddr()
	if address == "" {
		address = "127.0.0.1:8091"
	}
	listen, err := netip.ParseAddrPort(address)
	if err != nil || !listen.Addr().IsLoopback() || listen.Port() == 0 {
		return GatewaySettings{}, errors.New("server.http.addr must use a numeric loopback address and fixed port")
	}
	local, remote, report := gostCfg.GetLocalAuthToken(), controlCfg.GetAuthToken(), controlCfg.GetReportToken()
	if !validToken(local) || !validToken(remote) || !validToken(report) || local == remote || local == report || remote == report {
		return GatewaySettings{}, errors.New("configure distinct base64url gateway auth tokens of at least 32 random bytes")
	}
	wukong := gostCfg.GetWukong()
	return GatewaySettings{
		binary: gostCfg.GetBinary(), template: gostCfg.GetConfig(),
		Spool: repo.SpoolPath(spool), LocalToken: local, Address: listen,
		wukong:   gost.WukongConfig{Address: wukong.GetAddress(), Username: wukong.GetUsername(), Password: wukong.GetPassword(), Entries: wukong.GetEntries(), ProbeURL: wukong.GetProbeUrl(), BackupProbeURL: wukong.GetBackupProbeUrl(), ProbeTimeout: time.Duration(wukong.GetProbeTimeoutSeconds()) * time.Second},
		Recorder: recorderSettings,
		Auth:     acl.AuthConfig{Endpoint: controlCfg.GetUrl(), Token: remote},
		Report:   acl.ReportConfig{Endpoint: controlCfg.GetUrl(), Token: report, Timeout: reportTimeout},
	}, nil
}

func NewGatewayListener(address netip.AddrPort) (net.Listener, func(), error) {
	listener, err := net.Listen("tcp", address.String())
	if err != nil {
		return nil, nil, err
	}
	return listener, func() { _ = listener.Close() }, nil
}

func validToken(token string) bool {
	bytes, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(bytes) >= 32 && len(token) <= 256
}
