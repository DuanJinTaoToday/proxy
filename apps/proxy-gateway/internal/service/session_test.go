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
	handler.OnSession(func(client, _, _ string) string { return client })
	for _, tc := range []struct {
		username string
		want     bool
	}{
		{"user-sid-a01", true},
		{"user-sid-", false},
		{"user-sid-Upper", false},
		{"user-sid-a-sid-b", false},
		{"user-sid-" + strings.Repeat("a", 33), false},
		{"user-sid-a-sidtime-1", false},
	} {
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
