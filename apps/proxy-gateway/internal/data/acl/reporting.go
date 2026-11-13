package acl

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
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type HTTPReporter struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewHTTPReporter(endpoint, token string, timeout time.Duration, transport *http.Transport) (*HTTPReporter, error) {
	address, err := url.Parse(endpoint)
	if err != nil || address.Scheme != "https" || address.Hostname() == "" || address.User != nil ||
		address.RawQuery != "" || address.Fragment != "" {
		return nil, errors.New("report endpoint must be an HTTPS URL")
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	clientTransport := http.DefaultTransport
	if transport != nil {
		clientTransport = transport
	}
	return &HTTPReporter{endpoint: strings.TrimRight(endpoint, "/"), token: token, client: &http.Client{
		Transport: clientTransport,
		Timeout:   timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (r *HTTPReporter) Submit(ctx context.Context, batch *v1.SubmitBatchRequest) error {
	data, err := r.post(ctx, "/v1/reporting/batches", batch)
	if err != nil {
		return err
	}
	var ack v1.SubmitBatchResponse
	if json.Unmarshal(data, &ack) != nil || uint64(ack.Billable)+uint64(ack.NotBillable)+uint64(ack.Quarantined)+uint64(ack.Duplicates) != uint64(len(batch.GetRecords())) {
		return errors.New("invalid Recorder batch acknowledgement")
	}
	return nil
}

func (r *HTTPReporter) post(ctx context.Context, path string, input any) ([]byte, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.token)
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<10+1))
	if err != nil {
		return nil, err
	}
	switch res.StatusCode {
	case http.StatusOK:
		if len(data) > 16<<10 {
			return nil, errors.New("report acknowledgement exceeds size limit")
		}
		return data, nil
	case http.StatusRequestEntityTooLarge:
		return nil, recorder.ErrBatchTooLarge
	case http.StatusBadRequest, http.StatusForbidden:
		return nil, recorder.PermanentError{Err: fmt.Errorf("control plane rejected report (HTTP %d)", res.StatusCode)}
	default:
		return nil, fmt.Errorf("control plane report failed (HTTP %d)", res.StatusCode)
	}
}

func (r *HTTPReporter) ReportLifecycle(ctx context.Context, req *v1.ReportLifecycleRequest) error {
	return r.postProto(ctx, "/v1/reporting/lifecycle", req, &v1.BaseResp{})
}

func (r *HTTPReporter) ReportUnfinishedSessions(ctx context.Context, gateway, boot string, sessions []recorder.UnfinishedSession) error {
	req := &v1.ReportUnfinishedSessionsRequest{GatewayId: gateway, BootId: boot, Sessions: make([]*v1.UnfinishedSession, 0, len(sessions))}
	for _, session := range sessions {
		req.Sessions = append(req.Sessions, &v1.UnfinishedSession{
			SessionId: session.SessionID, ClientId: session.ClientID, LastRecordedAt: timestamppb.New(session.LastRecordedAt),
		})
	}
	path := "/v1/reporting/boots/" + url.PathEscape(boot) + "/unfinished-sessions"
	return r.postProto(ctx, path, req, &v1.BaseResp{})
}

func (r *HTTPReporter) CompleteReplay(ctx context.Context, gateway, boot string, count uint32, hasSessionRecords bool) error {
	req := &v1.CompleteReplayRequest{GatewayId: gateway, BootId: boot, UnfinishedSessionCount: count, HasSessionRecords: &hasSessionRecords}
	path := "/v1/reporting/boots/" + url.PathEscape(boot) + "/replay-complete"
	return r.postProto(ctx, path, req, &v1.BaseResp{})
}

func (r *HTTPReporter) postProto(ctx context.Context, path string, req, ack proto.Message) error {
	body, err := protojson.Marshal(req)
	if err != nil {
		return err
	}
	data, err := r.post(ctx, path, json.RawMessage(body))
	if err != nil {
		return err
	}
	return protojson.Unmarshal(data, ack)
}
