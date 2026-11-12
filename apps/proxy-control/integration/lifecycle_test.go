package integration

import (
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGatewayLifecycleRequiresReportIdentityAndPreservesExit(t *testing.T) {
	f := setupUsage(t)
	const path = "/v1/reporting/lifecycle"
	heartbeat := &v1.ReportLifecycleRequest{GatewayId: "gateway-a", BootId: "lifecycle"}
	testfixture.AssertStatus(t, authStateRequest(t, f.http, path, authToken, heartbeat), 401, "GATEWAY_UNAUTHENTICATED")
	testfixture.AssertStatus(t, authStateRequest(t, f.http, path, f.alice, heartbeat), 401, "GATEWAY_UNAUTHENTICATED")
	testfixture.AssertStatus(t, authStateRequest(t, f.http, path, reportToken, &v1.ReportLifecycleRequest{GatewayId: "gateway-b", BootId: "lifecycle"}), 403, "GATEWAY_FORBIDDEN")
	checkState := func(want string) {
		t.Helper()
		var row struct{ Status string }
		if err := f.db.Table("gateway_boot_lifecycle").Where("gateway_id = ? AND boot_id = ?", "gateway-a", "lifecycle").Take(&row).Error; err != nil || row.Status != want {
			t.Fatalf("lifecycle status=%q, want %q, error=%v", row.Status, want, err)
		}
	}
	report := func(req *v1.ReportLifecycleRequest) {
		t.Helper()
		readProto(t, authStateRequest(t, f.http, path, reportToken, req), &v1.ReportLifecycleResponse{})
	}
	execSQL(t, f.db, "INSERT INTO gateway_boots (gateway_id, boot_id, first_seen_at) VALUES ('gateway-a', 'lifecycle', now()-interval '15 seconds')")
	checkState("disconnected")
	report(heartbeat)
	checkState("running")
	execSQL(t, f.db, "UPDATE gateway_boots SET last_heartbeat_at=now()-interval '15 seconds' WHERE boot_id='lifecycle'")
	checkState("disconnected")
	report(heartbeat)
	checkState("running")
	exit := &v1.ReportLifecycleRequest{GatewayId: "gateway-a", BootId: "lifecycle", ExitedAt: timestamppb.New(time.Now())}
	report(exit)
	report(exit)
	report(heartbeat)
	checkState("exited")
}
