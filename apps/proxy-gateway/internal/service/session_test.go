package service_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
)

func TestExplicitSIDAuthenticatesBaseCredentials(t *testing.T) {
	handler := service.NewAuthRelay(authrelay.New(&control{load: snapshot()}, nil), localToken)
	for _, tc := range []struct {
		username string
		want     bool
		enabled  bool
	}{
		{"user-sid-a01", true, false},
		{"user-sid-", false, true},
		{"user-sid-Upper", false, true},
		{"user-sid-a-sid-b", false, true},
		{"user-sid-" + strings.Repeat("a", 33), false, true},
		{"user-sid-a-sidtime-1", false, false},
		{"user-sid-a-sidtime-1", true, true},
		{"user-sidtime-10", true, true},
		{"user-sidtime-", false, true},
		{"user-sidtime-0", false, true},
		{"user-sidtime-01", false, true},
		{"user-sidtime-1000000000", false, true},
		{"user-sidtime-1-sid-a", false, true},
		{"user-sid-a-sidtime-1-sidtime-2", false, true},
	} {
		handler.OnSession(func(client, _, _, _ string) string { return client }, tc.enabled)
		req := httptest.NewRequest(http.MethodPost, "/v1/gateway/auth", strings.NewReader(
			`{"username":"`+tc.username+`","password":"password","client":"127.0.0.1:3000"}`))
		req.Header.Set("Authorization", "Bearer "+localToken)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		var result struct{ OK bool }
		if res.Code != http.StatusOK || json.Unmarshal(res.Body.Bytes(), &result) != nil || result.OK != tc.want {
			t.Errorf("%q: got %d %s, want ok=%t", tc.username, res.Code, res.Body.String(), tc.want)
		}
	}
}
