package server

import (
	"io"
	stdhttp "net/http"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func RegisterUsage(srv *http.Server, usage *service.Usage) {
	v1.RegisterReportingServiceHTTPServer(srv, usage)
	v1.RegisterUsageServiceHTTPServer(srv, usage)
}

func ReportingDecoder(maxBytes int64) http.DecodeRequestFunc {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	return func(req *stdhttp.Request, body any) error {
		message, ok := body.(proto.Message)
		switch body.(type) {
		case *v1.SubmitBatchRequest, *v1.ReportLifecycleRequest, *v1.ReportUnfinishedSessionsRequest, *v1.CompleteReplayRequest:
		default:
			return DecodeProxyAuthRequest(req, body)
		}
		if !ok {
			return DecodeProxyAuthRequest(req, body)
		}
		if req.ContentLength > maxBytes {
			return errors.New(413, "BATCH_TOO_LARGE", "Recorder batch exceeds the body limit")
		}
		data, err := io.ReadAll(io.LimitReader(req.Body, maxBytes+1))
		if int64(len(data)) > maxBytes {
			return errors.New(413, "BATCH_TOO_LARGE", "Recorder batch exceeds the body limit")
		}
		if err != nil || protojson.Unmarshal(data, message) != nil {
			return errors.BadRequest("INVALID_BATCH", "invalid Recorder batch body")
		}
		return nil
	}
}

func UsageResponseEncoder(w stdhttp.ResponseWriter, req *stdhttp.Request, response any) error {
	switch response.(type) {
	case *v1.SubmitBatchResponse, *v1.DailyUsageResponse, *v1.QueryUsageResponse, *v1.TenantBalance:
		data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(response.(proto.Message))
		if err != nil {
			return err
		}
		contentType := "application/json"
		if req.Header.Get("Accept") == "application/protojson" {
			contentType = "application/protojson"
		}
		w.Header().Set("Content-Type", contentType)
		_, err = w.Write(data)
		return err
	default:
		return http.DefaultResponseEncoder(w, req, response)
	}
}
