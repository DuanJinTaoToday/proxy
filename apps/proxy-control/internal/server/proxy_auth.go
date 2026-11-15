package server

import (
	"bytes"
	"io"
	stdhttp "net/http"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport/http"
)

func DecodeProxyAuthRequest(req *stdhttp.Request, target any) error {
	switch target.(type) {
	case *v1.LoadAuthStateRequest, *v1.RefreshAuthStateRequest:
		const maxBody = 32 << 10
		if req.ContentLength > maxBody {
			return errors.New(413, "AUTH_REQUEST_TOO_LARGE", "authentication request exceeds the size limit")
		}
		body, err := io.ReadAll(io.LimitReader(req.Body, maxBody+1))
		if err != nil {
			return errors.BadRequest("INVALID_AUTH_REQUEST", "invalid authentication request")
		}
		if len(body) > maxBody {
			return errors.New(413, "AUTH_REQUEST_TOO_LARGE", "authentication request exceeds the size limit")
		}
		original := req.Body
		defer func() { req.Body = original }()
		req.Body = io.NopCloser(bytes.NewReader(body))
		if err := http.DefaultRequestDecoder(req, target); err != nil {
			return errors.BadRequest("INVALID_AUTH_REQUEST", "invalid authentication request")
		}
		return nil
	default:
		return http.DefaultRequestDecoder(req, target)
	}
}
