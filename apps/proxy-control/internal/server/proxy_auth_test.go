package server_test

import (
	"net/http"
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	httpservice "github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
)

func TestProxyCredentialRejectsUnauthorizedActorsBeforeStorage(t *testing.T) {
	accessService := testfixture.AuthAccess(t)
	handler := server.NewHTTPServer("127.0.0.1:0", httpservice.NewAccess(accessService))
	server.RegisterProxyAuth(handler, httpservice.NewProxyAuth(proxyauth.New(repo.NewProxyAuthRepository(nil), accessService)))
	for _, tc := range []struct{ actor, tenant string }{{"alice", "beta"}, {"bob", "alpha"}} {
		token := testfixture.Login(t, handler, tc.actor)
		for _, suffix := range []string{"", "/some-id:disable"} {
			reply := testfixture.Request(handler, "POST", "/v1/tenants/"+tc.tenant+"/proxy-credentials"+suffix, "{}", token)
			if reply.Code != http.StatusForbidden {
				t.Fatalf("expected forbidden, got %d", reply.Code)
			}
		}
	}
}
