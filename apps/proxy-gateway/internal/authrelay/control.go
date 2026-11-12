package authrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type HTTPControl struct {
	base   string
	token  string
	client *http.Client
}

func NewHTTPControl(endpoint, token string, transport ...*http.Transport) (*HTTPControl, error) {
	address, err := url.Parse(endpoint)
	if err != nil || address.Scheme != "https" || address.Hostname() == "" ||
		address.User != nil || address.RawQuery != "" || address.Fragment != "" {
		return nil, errors.New("control endpoint must be an HTTPS URL")
	}
	clientTransport := http.DefaultTransport
	if len(transport) != 0 && transport[0] != nil {
		clientTransport = transport[0]
	}
	return &HTTPControl{base: strings.TrimRight(endpoint, "/"), token: token, client: &http.Client{
		Transport:     clientTransport,
		Timeout:       syncTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *HTTPControl) Load(ctx context.Context, username, password string) (*v1.AuthSnapshot, error) {
	result := &v1.AuthSnapshot{}
	err := c.post(ctx, "/v1/gateway/auth-state/load", &v1.LoadAuthStateRequest{Username: username, Password: password}, result)
	return result, err
}

func (c *HTTPControl) Refresh(ctx context.Context, refs []string) ([]*v1.AuthSnapshot, error) {
	result := &v1.RefreshAuthStateResponse{}
	err := c.post(ctx, "/v1/gateway/auth-state/refresh", &v1.RefreshAuthStateRequest{References: refs}, result)
	return result.GetSnapshots(), err
}

func (c *HTTPControl) post(ctx context.Context, path string, input, output proto.Message) error {
	body, err := protojson.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/protojson")
	req.Header.Set("Accept", "application/protojson")
	req.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	switch response.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrGatewayDenied
	case http.StatusNotFound:
		if _, ok := input.(*v1.LoadAuthStateRequest); ok {
			var failure struct{ Reason string }
			_ = json.NewDecoder(io.LimitReader(response.Body, 4<<10)).Decode(&failure)
			switch failure.Reason {
			case "CREDENTIAL_DISABLED":
				return ErrCredentialDisabled
			case "CREDENTIAL_EXPIRED":
				return ErrCredentialExpired
			default:
				return ErrCredentialDenied
			}
		}
		return errors.New("refresh route unavailable")
	case http.StatusOK:
		const limit = 2 << 20
		body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
		if err != nil {
			return err
		}
		if len(body) > limit {
			return errors.New("authentication state response too large")
		}
		return protojson.Unmarshal(body, output)
	default:
		return fmt.Errorf("authentication state returned HTTP %d", response.StatusCode)
	}
}
